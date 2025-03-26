package core

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/concurrent"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/metrics"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/rollupcontract"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/srv"
	scTypes "github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
)

type ProposerStorage interface {
	SetProvedStateRoot(ctx context.Context, stateRoot common.Hash) error

	TryGetNextProposalData(ctx context.Context) (*scTypes.ProposalData, error)

	SetBatchAsProposed(ctx context.Context, id scTypes.BatchId) error
}

type RpcBlockFetcher interface {
	GetBlock(ctx context.Context, shardId types.ShardId, blockId any, fullTx bool) (*jsonrpc.RPCBlock, error)
}

type ProposerMetrics interface {
	metrics.BasicMetrics
	RecordProposerTxSent(ctx context.Context, proposalData *scTypes.ProposalData)
}

type proposer struct {
	storage   ProposerStorage
	rpcClient RpcBlockFetcher

	rollupContractWrapper rollupcontract.Wrapper
	config                ProposerConfig

	metrics ProposerMetrics
	logger  logging.Logger
}

type ProposerConfig struct {
	ProposingInterval time.Duration
}

func NewDefaultProposerConfig() ProposerConfig {
	return ProposerConfig{
		ProposingInterval: 10 * time.Second,
	}
}

// NewProposer creates a proposer instance.
func NewProposer(
	config ProposerConfig,
	storage ProposerStorage,
	contractWrapper rollupcontract.Wrapper,
	rpcClient RpcBlockFetcher,
	metrics ProposerMetrics,
	logger logging.Logger,
) (*proposer, error) {
	p := &proposer{
		storage:               storage,
		rollupContractWrapper: contractWrapper,
		rpcClient:             rpcClient,
		config:                config,
		metrics:               metrics,
	}

	p.logger = srv.WorkerLogger(logger, p)

	return p, nil
}

func (*proposer) Name() string {
	return "proposer"
}

func (p *proposer) Run(ctx context.Context, started chan<- struct{}) error {
	close(started)

	if err := p.updateStoredStateRootFromL1(ctx); err != nil {
		return fmt.Errorf("initial proved state root update failed: %w", err)
	}

	concurrent.RunTickerLoop(ctx, p.config.ProposingInterval,
		func(ctx context.Context) {
			if err := p.updateStateIfReady(ctx); err != nil {
				p.logger.Error().Err(err).Msg("error during proved batches proposing")
				p.metrics.RecordError(ctx, p.Name())
				return
			}
		},
	)

	return nil
}

func (p *proposer) updateStoredStateRootFromL1(ctx context.Context) error {
	p.logger.Info().Msg("updating stored state root from L1")

	latestStateRoot, err := p.getLatestProvedStateRoot(ctx)
	if err != nil {
		return err
	}

	if latestStateRoot == common.EmptyHash {
		p.logger.Warn().
			Err(err).
			Stringer("latestStateRoot", latestStateRoot).
			Msg("L1 state root is not initialized, genesis state root will be used")

		genesisBlock, err := p.rpcClient.GetBlock(ctx, types.MainShardId, "earliest", false)
		if err != nil {
			return err
		}
		latestStateRoot = genesisBlock.Hash
	}

	if err := p.storage.SetProvedStateRoot(ctx, latestStateRoot); err != nil {
		return fmt.Errorf("failed set proved state root: %w", err)
	}

	p.logger.Info().
		Stringer("stateRoot", latestStateRoot).
		Msg("stored state root updated")
	return nil
}

// updateStateIfReady checks if there is new proved state root is ready to be submitted to L1 and
// creates L1 transaction if so.
func (p *proposer) updateStateIfReady(ctx context.Context) error {
	data, err := p.storage.TryGetNextProposalData(ctx)
	if err != nil {
		return fmt.Errorf("failed get next proposal data: %w", err)
	}
	if data == nil {
		p.logger.Debug().Msg("no batches to propose")
		return nil
	}

	err = p.updateState(ctx, data)
	if err != nil {
		if errors.Is(err, rollupcontract.ErrBatchAlreadyFinalized) {
			// someone has already updated, we need to refetch state from contract
			p.logger.Warn().Msg("batch is already committed, skipping UpdateState tx")
			if err := p.updateStoredStateRootFromL1(ctx); err != nil {
				return err
			}
		} else {
			return fmt.Errorf("failed to send proof to L1 for batch with id=%s: %w", data.BatchId, err)
		}
	}

	err = p.storage.SetBatchAsProposed(ctx, data.BatchId)
	if err != nil {
		return fmt.Errorf("failed set batch with id=%s as proposed: %w", data.BatchId, err)
	}
	return nil
}

func (p *proposer) getLatestProvedStateRoot(ctx context.Context) (common.Hash, error) {
	finalizedBatchIndex, err := p.rollupContractWrapper.FinalizedBatchIndex(ctx)
	if err != nil {
		return common.EmptyHash, err
	}

	return p.rollupContractWrapper.FinalizedStateRoot(ctx, finalizedBatchIndex)
}

func (p *proposer) updateState(
	ctx context.Context,
	proposalData *scTypes.ProposalData,
) error {
	// TODO: populate with actual data
	validityProof := []byte{0x0A, 0x0B, 0x0C}
	publicData := rollupcontract.INilRollupPublicDataInfo{
		Placeholder1: []byte{0x07, 0x08, 0x09},
		Placeholder2: []byte{0x07, 0x08, 0x09},
	}

	p.logger.Info().
		Stringer("blockHash", proposalData.MainShardBlockHash).
		Int("txCount", len(proposalData.Transactions)).
		Msg("calling UpdateState L1 method")

	if err := p.rollupContractWrapper.UpdateState(
		ctx,
		proposalData.BatchId.String(),
		proposalData.OldProvedStateRoot,
		proposalData.NewProvedStateRoot,
		validityProof,
		publicData,
	); err != nil {
		return fmt.Errorf("failed to update state: %w", err)
	}

	p.metrics.RecordProposerTxSent(ctx, proposalData)

	return nil
}
