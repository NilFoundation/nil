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
	if errors.Is(err, scTypes.ErrStateRootNotInitialized) {
		p.logger.Warn().Msg("state root has not been initialized yet, awaiting initialization by the stateRootSyncer")
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed get next proposal data: %w", err)
	}
	if data == nil {
		p.logger.Debug().Msg("no batches to propose")
		return nil
	}

	if err := p.updateState(ctx, data); err != nil {
		return p.handleUpdateStateError(ctx, data.BatchId, err)
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

func (p *proposer) handleUpdateStateError(ctx context.Context, batchId scTypes.BatchId, err error) error {
	switch {
	case errors.Is(err, rollupcontract.ErrBatchAlreadyFinalized) ||
		errors.Is(err, rollupcontract.ErrBatchNotCommitted) ||
		errors.Is(err, rollupcontract.ErrOldStateRootMismatch) ||
		errors.Is(err, rollupcontract.ErrL1MessageHashMismatch):
		// for some reason, we attempted to submit the data in which some parts are not what contract
		// expects (already proved batch, submitted old root doesn't match the one in contract, etc),
		// sync the latest proved root with the L1 contract.
		p.logger.Warn().Stringer(logging.FieldBatchId, batchId).
			Err(err).Msg("proposed data seems outdated, resetting state with L1")
		if err := p.resetter.LaunchResetToL1WithSuspension(ctx, p); err != nil {
			return fmt.Errorf("error resetting state from L1, batchId=%s: %w",
				batchId, err)
		}
	case errors.Is(err, rollupcontract.ErrInvalidBatchIndex) ||
		errors.Is(err, rollupcontract.ErrInvalidVersionedHash) ||
		errors.Is(err, rollupcontract.ErrInvalidOldStateRoot) ||
		errors.Is(err, rollupcontract.ErrInvalidNewStateRoot) ||
		errors.Is(err, rollupcontract.ErrInvalidValidityProof) ||
		errors.Is(err, rollupcontract.ErrEmptyDataProofs) ||
		errors.Is(err, rollupcontract.ErrDataProofsAndBlobCountMismatch) ||
		errors.Is(err, rollupcontract.ErrIncorrectDataProofSize) ||
		errors.Is(err, rollupcontract.ErrInvalidPublicDataInfo) ||
		errors.Is(err, rollupcontract.ErrInvalidDataProofItem) ||
		errors.Is(err, rollupcontract.ErrInvalidPublicInputForProof) ||
		errors.Is(err, rollupcontract.ErrCallPointEvaluationPrecompileFailed) ||
		errors.Is(err, rollupcontract.ErrUnexpectedPointEvaluationPrecompileOutput):
		// NOTE: this shouldn't happen in prod setting
		p.logger.Error().Stringer(logging.FieldBatchId, batchId).
			Err(err).Msg("data was corrupted or initially created in a wrong way")
		if err := p.resetter.LaunchResetToL1WithSuspension(ctx, p); err != nil {
			return fmt.Errorf("error resetting state from L1, batchId=%s: %w",
				batchId, err)
		}
	default:
		return err
	}
	return nil
}
