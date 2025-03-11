package core

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/concurrent"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/metrics"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/rollupcontract"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/srv"
	scTypes "github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto/kzg4844"
	"github.com/rs/zerolog"
)

type ProposerStorage interface {
	TryGetProvedStateRoot(ctx context.Context) (*common.Hash, error)

	SetProvedStateRoot(ctx context.Context, stateRoot common.Hash) error

	TryGetNextProposalData(ctx context.Context) (*scTypes.ProposalData, error)

	SetBlockAsProposed(ctx context.Context, id scTypes.BlockId) error
}

type ProposerMetrics interface {
	metrics.BasicMetrics
	RecordProposerTxSent(ctx context.Context, proposalData *scTypes.ProposalData)
}

type proposer struct {
	storage     ProposerStorage
	retryRunner common.RetryRunner
	ethClient   rollupcontract.EthClient

	rollupContractWrapper *rollupcontract.Wrapper
	params                ProposerParams

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

func NewDefaultProposerParams() ProposerParams {
	return ProposerParams{
		Endpoint:          "http://rpc2.sepolia.org",
		PrivateKey:        "0000000000000000000000000000000000000000000000000000000000000001",
		ContractAddress:   "0x796baf7E572948CD0cbC374f345963bA433b47a2",
		ProposingInterval: 10 * time.Second,
		EthClientTimeout:  10 * time.Second,
	}
}

func NewProposer(
	ctx context.Context,
	params ProposerParams,
	storage ProposerStorage,
	ethClient rollupcontract.EthClient,
	metrics ProposerMetrics,
	logger zerolog.Logger,
) (*proposer, error) {
	retryRunner := common.NewRetryRunner(
		common.RetryConfig{
			ShouldRetry: common.LimitRetries(5),
			NextDelay:   common.DelayExponential(100*time.Millisecond, time.Second),
		},
		logger,
	)

	p := &proposer{
		storage:     storage,
		ethClient:   ethClient,
		params:      params,
		retryRunner: retryRunner,
		metrics:     metrics,
	}

	p.logger = srv.WorkerLogger(logger, p)

	err := p.initRollupContractWrapper(ctx)
	if err != nil {
		logger.Error().Err(err).Msgf("rollup contract wrapper is not initialized")
	}

	return p, nil
}

func (*proposer) Name() string {
	return "proposer"
}

func (p *proposer) Run(ctx context.Context, started chan<- struct{}) error {
	if err := p.initializeProvedStateRoot(ctx); err != nil {
		return err
	}

	close(started)

	concurrent.RunTickerLoop(ctx, p.params.ProposingInterval,
		func(ctx context.Context) {
			if err := p.proposeNextBlock(ctx); err != nil {
				p.logger.Error().Err(err).Msg("error during proposed blocks submission")
				p.metrics.RecordError(ctx, p.Name())
				return
			}
		},
	)

	return nil
}

func (p *proposer) initRollupContractWrapper(ctx context.Context) error {
	if p.rollupContractWrapper != nil {
		return nil
	}

	var err error
	p.rollupContractWrapper, err = rollupcontract.NewWrapper(ctx, p.params.ContractAddress, p.params.PrivateKey, p.ethClient, p.params.EthClientTimeout, p.logger)
	if err != nil {
		return fmt.Errorf("failed to create rollup contract wrapper: %w", err)
	}
	return nil
}

func (p *proposer) initializeProvedStateRoot(ctx context.Context) error {
	storedStateRoot, err := p.storage.TryGetProvedStateRoot(ctx)
	if err != nil {
		return fmt.Errorf("failed to get proved state root from the storage: %w", err)
	}
	var storedRootStr string
	if storedStateRoot != nil {
		storedRootStr = storedStateRoot.String()
	} else {
		storedRootStr = "nil"
	}

	latestStateRoot, err := p.getLatestProvedStateRoot(ctx)

	switch {
	case err != nil:
		p.logger.Warn().
			Err(err).
			Str("storedStateRoot", storedRootStr).
			Msg("failed to get state root from L1, stored proved state root will be used")

	case storedStateRoot == nil:
		p.logger.Info().
			Stringer("latestStateRoot", latestStateRoot).
			Msg("proved state root is not initialized, value from L1 will be used")
		if err := p.updateStoredStateRoot(ctx, latestStateRoot); err != nil {
			return err
		}

	case *storedStateRoot != latestStateRoot:
		p.logger.Warn().
			Str("storedStateRoot", storedRootStr).
			Stringer("latestStateRoot", latestStateRoot).
			Msg("stored proved state root value is invalid, local storage will be reset")

		p.logger.Warn().Msg("resetting TaskStorage and BlockStorage")
		// todo: reset TaskStorage and BlockStorage before starting Aggregator, TaskScheduler and TaskListener

		if err := p.updateStoredStateRoot(ctx, latestStateRoot); err != nil {
			return err
		}

	default:
		p.logger.Info().Stringer("stateRoot", storedStateRoot).Msg("proved state root value is valid")
	}

	p.logger.Info().
		Stringer("stateRoot", latestStateRoot).
		Msg("proposer is initialized")
	return nil
}

func (p *proposer) updateStoredStateRoot(ctx context.Context, stateRoot common.Hash) error {
	err := p.storage.SetProvedStateRoot(ctx, stateRoot)
	if err != nil {
		return fmt.Errorf("failed to set proved state root: %w", err)
	}
	return nil
}

func (p *proposer) proposeNextBlock(ctx context.Context) error {
	if p.rollupContractWrapper == nil {
		err := p.initializeProvedStateRoot(ctx)
		if err != nil {
			return err
		}
	}
	data, err := p.storage.TryGetNextProposalData(ctx)
	if err != nil {
		return fmt.Errorf("failed to get next block to propose: %w", err)
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
	err = p.storage.SetBlockAsProposed(ctx, blockId)
	if err != nil {
		return fmt.Errorf("failed set block with hash=%s as proposed: %w", data.MainShardBlockHash, err)
	}
	return nil
}

func (p *proposer) getLatestProvedStateRoot(ctx context.Context) (common.Hash, error) {
	if err := p.initRollupContractWrapper(ctx); err != nil {
		return common.EmptyHash, err
	}
	var finalizedBatchIndex string
	err := p.retryRunner.Do(ctx, func(context.Context) error {
		var err error
		finalizedBatchIndex, err = p.rollupContractWrapper.FinalizedBatchIndex(ctx)
		return err
	})
	if err != nil {
		return common.EmptyHash, err
	}

	var latestProvedState [32]byte
	err = p.retryRunner.Do(ctx, func(context.Context) error {
		var err error
		latestProvedState, err = p.rollupContractWrapper.StateRoots(ctx, finalizedBatchIndex)
		return err
	})

	return latestProvedState, err
}

func (p *proposer) commitBatch(ctx context.Context, blobs []kzg4844.Blob, batchIndexInBlobStorage string) (*ethtypes.Transaction, bool, error) {
	var tx *ethtypes.Transaction
	batchTxSkipped := false
	err := p.retryRunner.Do(ctx, func(context.Context) error {
		var err error
		tx, err = p.rollupContractWrapper.CommitBatch(ctx, blobs, batchIndexInBlobStorage)
		if errors.Is(err, rollupcontract.ErrBatchAlreadyCommitted) {
			p.logger.Warn().Msg("batch is already committed, skipping blob tx")
			batchTxSkipped = true
			return nil
		}
		return err
	})
	if err != nil {
		return nil, false, fmt.Errorf("failed to upload blob: %w", err)
	}

	if !batchTxSkipped {
		p.logger.Info().
			Hex("txHash", tx.Hash().Bytes()).
			Int("gasLimit", int(tx.Gas())).
			Int("blobGasLimit", int(tx.BlobGas())).
			Int("cost", int(tx.Cost().Uint64())).
			Any("blobHashes", tx.BlobHashes()).
			Msg("blob transaction sent")

		receipt, err := p.rollupContractWrapper.WaitForReceipt(ctx, tx.Hash())
		if err != nil {
			return nil, false, err
		}
		if receipt == nil {
			return nil, false, errors.New("CommitBatch tx mining timeout exceeded")
		}
		if receipt.Status != ethtypes.ReceiptStatusSuccessful {
			return nil, false, errors.New("CommitBatch tx failed")
		}
	}

	return tx, batchTxSkipped, nil
}

func (p *proposer) updateState(ctx context.Context, tx *ethtypes.Transaction, data *scTypes.ProposalData, batchIndexInBlobStorage string) error {
	blobTxSidecar := tx.BlobTxSidecar()
	dataProofs, err := rollupcontract.ComputeDataProofs(blobTxSidecar)
	if err != nil {
		return err
	}

	// TODO: populate with actual data
	validityProof := []byte{0x0A, 0x0B, 0x0C}

	p.logger.Info().
		Stringer("blockHash", data.MainShardBlockHash).
		Int("txCount", len(data.Transactions)).
		Msg("calling UpdateState L1 method")

	updateTxSkipped := false
	err = p.retryRunner.Do(ctx, func(context.Context) error {
		var err error
		tx, err = p.rollupContractWrapper.UpdateState(
			ctx,
			batchIndexInBlobStorage,
			data.OldProvedStateRoot,
			data.NewProvedStateRoot,
			dataProofs,
			blobTxSidecar.BlobHashes(),
			validityProof,
			rollupcontract.INilRollupPublicDataInfo{
				Placeholder1: []byte{0x07, 0x08, 0x09},
				Placeholder2: []byte{0x07, 0x08, 0x09},
			},
		)
		if errors.Is(err, rollupcontract.ErrBatchAlreadyFinalized) {
			p.logger.Warn().Msg("batch is already committed, skipping UpdateState tx")
			updateTxSkipped = true
			return nil
		}
		return err
	})
	if err != nil {
		return fmt.Errorf("failed to update state: %w", err)
	}

	if !updateTxSkipped {
		p.logger.Info().
			Hex("txHash", tx.Hash().Bytes()).
			Int("gasLimit", int(tx.Gas())).
			Int("cost", int(tx.Cost().Uint64())).
			Msg("UpdateState transaction sent")

		p.metrics.RecordProposerTxSent(ctx, data)
	}

	return nil
}

func (p *proposer) sendProof(ctx context.Context, data *scTypes.ProposalData) error {
	// TODO: populate with actual data
	blobs := []kzg4844.Blob{{0x01}, {0x02}, {0x03}}
	batchIndexInBlobStorage := "0x0000000000000000000000000000000000000000000000000000000000000001"

	tx, _, err := p.commitBatch(ctx, blobs, batchIndexInBlobStorage)
	if err != nil {
		return err
	}

	if err := p.updateState(ctx, tx, data, batchIndexInBlobStorage); err != nil {
		return err
	}

	return nil
}
