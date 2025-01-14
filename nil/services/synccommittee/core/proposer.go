package core

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/concurrent"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/metrics"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/rollupcontract"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/storage"
	scTypes "github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/rs/zerolog"
)

type Proposer struct {
	blockStorage storage.BlockStorage
	retryRunner  common.RetryRunner

	rollupContract *rollupcontract.Wrapper
	params         *ProposerParams

	metrics ProposerMetrics
	logger  zerolog.Logger
}

type ProposerParams struct {
	Endpoint          string
	PrivateKey        string
	ContractAddress   string
	ProposingInterval time.Duration
	EthClientTimeout  time.Duration
}

type ProposerMetrics interface {
	metrics.BasicMetrics
	RecordProposerTxSent(ctx context.Context, proposalData *scTypes.ProposalData)
}

func NewDefaultProposerParams() *ProposerParams {
	return &ProposerParams{
		Endpoint:          "http://rpc2.sepolia.org",
		PrivateKey:        "0000000000000000000000000000000000000000000000000000000000000001",
		ContractAddress:   "0xB8E280a085c87Ed91dd6605480DD2DE9EC3699b4",
		ProposingInterval: 10 * time.Second,
		EthClientTimeout:  10 * time.Second,
	}
}

func NewProposer(
	ctx context.Context,
	params *ProposerParams,
	blockStorage storage.BlockStorage,
	ethClient rollupcontract.EthClient,
	metrics ProposerMetrics,
	logger zerolog.Logger,
) (*Proposer, error) {
	retryRunner := common.NewRetryRunner(
		common.RetryConfig{
			ShouldRetry: common.LimitRetries(5),
			NextDelay:   common.DelayExponential(100*time.Millisecond, time.Second),
		},
		logger,
	)

	rollupContract, err := rollupcontract.NewWrapper(ctx, params.ContractAddress, params.PrivateKey, ethClient, params.EthClientTimeout)
	if err != nil {
		return nil, err
	}

	p := Proposer{
		blockStorage:   blockStorage,
		rollupContract: rollupContract,
		params:         params,
		retryRunner:    retryRunner,
		metrics:        metrics,
		logger:         logger,
	}

	return &p, nil
}

func (p *Proposer) Run(ctx context.Context) error {
	shouldResetStorage, err := p.initializeProvedStateRoot(ctx)
	if err != nil {
		return err
	}

	if shouldResetStorage {
		p.logger.Warn().Msg("resetting TaskStorage and BlockStorage")
		// todo: reset TaskStorage and BlockStorage before starting Aggregator, TaskScheduler and TaskListener
	}

	concurrent.RunTickerLoop(ctx, p.params.ProposingInterval,
		func(ctx context.Context) {
			if err := p.proposeNextBlock(ctx); err != nil {
				p.logger.Error().Err(err).Msg("error during proved blocks proposing")
				p.metrics.RecordError(ctx, "proposer")
				return
			}
		},
	)

	return nil
}

func (p *Proposer) initializeProvedStateRoot(ctx context.Context) (shouldResetStorage bool, err error) {
	storedStateRoot, err := p.blockStorage.TryGetProvedStateRoot(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to check if proved state root is initialized: %w", err)
	}

	latestStateRoot, err := p.getLatestProvedStateRoot(ctx)
	if err != nil {
		// TODO return error after enable local L1 endpoint
		p.logger.Error().Err(err).Msg("failed get current contract state root, set 0")
	}

	switch {
	case storedStateRoot == nil:
		p.logger.Info().
			Stringer("latestStateRoot", latestStateRoot).
			Msg("proved state root is not initialized, value from L1 will be used")
	case *storedStateRoot != latestStateRoot:
		p.logger.Warn().
			Stringer("storedStateRoot", storedStateRoot).
			Stringer("latestStateRoot", latestStateRoot).
			Msg("proved state root value is invalid, local storage will be reset")
		shouldResetStorage = true
	default:
		p.logger.Info().Stringer("stateRoot", storedStateRoot).Msg("proved state root value is valid")
	}

	if storedStateRoot == nil || *storedStateRoot != latestStateRoot {
		err = p.blockStorage.SetProvedStateRoot(ctx, latestStateRoot)
		if err != nil {
			return false, fmt.Errorf("failed set proved state root: %w", err)
		}
	}

	p.logger.Info().
		Stringer("stateRoot", latestStateRoot).
		Msg("proposer is initialized")
	return shouldResetStorage, nil
}

func (p *Proposer) proposeNextBlock(ctx context.Context) error {
	data, err := p.blockStorage.TryGetNextProposalData(ctx)
	if err != nil {
		return fmt.Errorf("failed get next block to propose: %w", err)
	}
	if data == nil {
		p.logger.Debug().Msg("no block to propose")
		return nil
	}

	err = p.sendProof(ctx, data)
	if err != nil {
		return fmt.Errorf("failed to send proof to L1 for block with hash=%s: %w", data.MainShardBlockHash, err)
	}

	blockId := scTypes.NewBlockId(types.MainShardId, data.MainShardBlockHash)
	err = p.blockStorage.SetBlockAsProposed(ctx, blockId)
	if err != nil {
		return fmt.Errorf("failed set block with hash=%s as proposed: %w", data.MainShardBlockHash, err)
	}
	return nil
}

func (p *Proposer) getLatestProvedStateRoot(ctx context.Context) (common.Hash, error) {
	var finalizedBatchIndex *big.Int
	err := p.retryRunner.Do(ctx, func(context.Context) error {
		var err error
		finalizedBatchIndex, err = p.rollupContract.FinalizedBatchIndex(ctx)
		return err
	})
	if err != nil {
		return common.EmptyHash, err
	}

	finalizedBatchIndex.Sub(finalizedBatchIndex, big.NewInt(1))
	if finalizedBatchIndex.Cmp(big.NewInt(0)) == -1 {
		return common.EmptyHash, errors.New("value returned from FinalizedBatchIndex is less than 1")
	}

	var latestProvedState [32]byte
	err = p.retryRunner.Do(ctx, func(context.Context) error {
		var err error
		latestProvedState, err = p.rollupContract.StateRoots(ctx, finalizedBatchIndex)
		return err
	})

	return latestProvedState, err
}

func (p *Proposer) sendProof(ctx context.Context, data *scTypes.ProposalData) error {
	if data.OldProvedStateRoot.Empty() || data.NewProvedStateRoot.Empty() {
		return errors.New("empty hash for state update transaction")
	}

	p.logger.Info().
		Stringer("blockHash", data.MainShardBlockHash).
		Int("txCount", len(data.Transactions)).
		Msg("sending proof to L1")

	proof := make([]byte, 0)
	batchIndexInBlobStorage := big.NewInt(0)

	var tx *ethtypes.Transaction
	err := p.retryRunner.Do(ctx, func(context.Context) error {
		var err error
		tx, err = p.rollupContract.ProofBatch(ctx, data.OldProvedStateRoot, data.NewProvedStateRoot, proof, batchIndexInBlobStorage)
		return err
	})
	if err != nil {
		return fmt.Errorf("failed to update state (eth_sendRawTransaction): %w", err)
	}

	p.logger.Info().
		Hex("txHash", tx.Hash().Bytes()).
		Int("gasLimit", int(tx.Gas())).
		Int("blobGasLimit", int(tx.BlobGas())).
		Int("cost", int(tx.Cost().Uint64())).
		Msg("rollup transaction sent")

	// TODO send bloob with transactions and KZG proof

	p.metrics.RecordProposerTxSent(ctx, data)
	return nil
}
