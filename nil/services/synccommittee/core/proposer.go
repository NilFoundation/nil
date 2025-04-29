package core

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/concurrent"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/services/synccommittee/core/reset"
	"github.com/NilFoundation/nil/nil/services/synccommittee/core/rollupcontract"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/log"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/srv"
	scTypes "github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	"github.com/rs/zerolog"
)

type ProposerStorage interface {
	SetProvedStateRoot(ctx context.Context, stateRoot common.Hash) error

	TryGetNextProposalData(ctx context.Context) (*scTypes.ProposalData, error)

	SetBatchAsProposed(ctx context.Context, id scTypes.BatchId) error
}

type ProposerMetrics interface {
	srv.WorkerMetrics
	RecordStateUpdated(ctx context.Context, proposalData *scTypes.ProposalData)
}

type proposer struct {
	concurrent.Suspendable

	storage               ProposerStorage
	resetter              *reset.StateResetLauncher
	rollupContractWrapper rollupcontract.Wrapper
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

	iteration := srv.NewWorkerIteration(logger, metrics, p.Name(), p.updateStateIfReady)
	p.Suspendable = concurrent.NewSuspendable(iteration.Run, config.ProposingInterval)
	p.logger = srv.WorkerLogger(logger, p)

	return p, nil
}

func (*proposer) Name() string {
	return "proposer"
}

// updateStateIfReady checks if there is new proved state root is ready to be submitted to L1 and
// creates L1 transaction if so.
func (p *proposer) updateStateIfReady(ctx context.Context) error {
	data, err := p.storage.TryGetNextProposalData(ctx)
	if errors.Is(err, scTypes.ErrLocalStateRootNotInitialized) {
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

	updateStateData := scTypes.NewUpdateStateData(
		proposalData,
		[]byte{0x0A, 0x0B, 0x0C},
		common.EmptyHash,
		0,
		common.EmptyHash,
	)

	log.NewStateUpdateEvent(p.logger, zerolog.InfoLevel, updateStateData).Msg("calling UpdateState L1 method")

	if err := p.rollupContractWrapper.UpdateState(ctx, updateStateData); err != nil {
		log.NewStateUpdateEvent(p.logger, zerolog.ErrorLevel, updateStateData).
			Err(err).Msg("failed to call UpdateState L1 method")

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
