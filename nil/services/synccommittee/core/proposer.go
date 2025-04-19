package core

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/concurrent"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/services/synccommittee/core/reset"
	"github.com/NilFoundation/nil/nil/services/synccommittee/core/rollupcontract"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/metrics"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/srv"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/storage"
	scTypes "github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
)

type ProposerStorage interface {
	SetProvedStateRoot(ctx context.Context, stateRoot common.Hash) error

	TryGetNextProposalData(ctx context.Context) (*scTypes.ProposalData, error)

	SetBatchAsProposed(ctx context.Context, id scTypes.BatchId) error
}

type ProposerMetrics interface {
	metrics.BasicMetrics
	RecordStateUpdated(ctx context.Context, proposalData *scTypes.ProposalData)
}

type proposer struct {
	storage               ProposerStorage
	resetter              *reset.StateResetLauncher
	rollupContractWrapper rollupcontract.Wrapper
	workerAction          *concurrent.Suspendable
	metrics               ProposerMetrics
	logger                logging.Logger
}

var _ reset.PausableComponent = (*proposer)(nil)

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
	resetter *reset.StateResetLauncher,
	metrics ProposerMetrics,
	logger logging.Logger,
) (*proposer, error) {
	p := &proposer{
		storage:               storage,
		rollupContractWrapper: contractWrapper,
		resetter:              resetter,
		metrics:               metrics,
	}

	p.workerAction = concurrent.NewSuspendable(p.runIteration, config.ProposingInterval)
	p.logger = srv.WorkerLogger(logger, p)

	return p, nil
}

func (*proposer) Name() string {
	return "proposer"
}

func (p *proposer) Run(ctx context.Context, started chan<- struct{}) error {
	p.logger.Info().Msg("starting proposer")

	err := p.workerAction.Run(ctx, started)

	if err == nil || errors.Is(err, context.Canceled) {
		p.logger.Info().Msg("proposer stopped")
	} else {
		p.logger.Error().Err(err).Msg("error running proposer, stopped")
	}

	return err
}

func (p *proposer) Pause(ctx context.Context) error {
	paused, err := p.workerAction.Pause(ctx)
	if err != nil {
		return err
	}
	if paused {
		p.logger.Info().Msg("proposer paused")
	} else {
		p.logger.Warn().Msg("trying to pause proser, but it's already paused")
	}
	return nil
}

func (p *proposer) Resume(ctx context.Context) error {
	resumed, err := p.workerAction.Resume(ctx)
	if err != nil {
		return err
	}
	if resumed {
		p.logger.Info().Msg("proposer resumed")
	} else {
		p.logger.Warn().Msg("trying to resume proser, but it's already resumed")
	}
	return nil
}

func (p *proposer) runIteration(ctx context.Context) {
	if err := p.updateStateIfReady(ctx); err != nil {
		p.logger.Error().Err(err).Msg("error during proved batches proposing")
		p.metrics.RecordError(ctx, p.Name())
	}
}

// updateStateIfReady checks if there is new proved state root is ready to be submitted to L1 and
// creates L1 transaction if so.
func (p *proposer) updateStateIfReady(ctx context.Context) error {
	data, err := p.storage.TryGetNextProposalData(ctx)
	if errors.Is(err, storage.ErrStateRootNotInitialized) {
		p.logger.Warn().Msg("state root has not been initialized yet, awaiting initialization by the aggregator")
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed get next proposal data: %w", err)
	}
	if data == nil {
		p.logger.Debug().Msg("no batches to propose")
		return nil
	}

	err = p.updateState(ctx, data)
	if err != nil {
		if !errors.Is(err, rollupcontract.ErrBatchAlreadyFinalized) {
			return fmt.Errorf("failed to send proof to L1 for batch with id=%s: %w", data.BatchId, err)
		}

		// another actor has already sent an update for this batch, we need to refetch state from contract
		p.logger.Warn().Msg("batch is already finalized, skipping UpdateState tx, syncing state with L1")
		return p.resetter.LaunchResetToL1WithSuspension(ctx, p)
	}

	err = p.storage.SetBatchAsProposed(ctx, data.BatchId)
	if err != nil {
		return fmt.Errorf("failed set batch with id=%s as proposed: %w", data.BatchId, err)
	}
	return nil
}

func (p *proposer) updateState(
	ctx context.Context,
	proposalData *scTypes.ProposalData,
) error {
	// TODO: populate with actual data
	validityProof := []byte{0x0A, 0x0B, 0x0C}
	publicData := rollupcontract.INilRollupPublicDataInfo{
		L2Tol1Root:    common.Hash{},
		MessageCount:  big.NewInt(0),
		L1MessageHash: common.Hash{},
	}

	p.logger.Info().
		Stringer(logging.FieldBatchId, proposalData.BatchId).
		Hex("OldProvedStateRoot", proposalData.OldProvedStateRoot.Bytes()).
		Hex("NewProvedStateRoot", proposalData.NewProvedStateRoot.Bytes()).
		Int("blobsCount", len(proposalData.DataProofs)).
		Msg("calling UpdateState L1 method")

	if err := p.rollupContractWrapper.UpdateState(
		ctx,
		proposalData.BatchId.String(),
		proposalData.DataProofs,
		proposalData.OldProvedStateRoot,
		proposalData.NewProvedStateRoot,
		validityProof,
		publicData,
	); err != nil {
		return fmt.Errorf("failed to update state: %w", err)
	}

	p.metrics.RecordStateUpdated(ctx, proposalData)

	return nil
}
