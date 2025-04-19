package reset

import (
	"context"
	"fmt"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/logging"
	scTypes "github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
)

type BatchResetter interface {
	// ResetBatchesRange resets Sync Committee's block processing progress
	// to a point preceding batch with the specified ID.
	ResetBatchesRange(
		ctx context.Context, firstBatchToPurge scTypes.BatchId,
	) (purgedBatches []scTypes.BatchId, err error)

	// ResetAllBatches resets Sync Committee's progress for all batches.
	ResetAllBatches(ctx context.Context) error

	// SetProvedStateRoot sets the last proved state root during full reset
	SetProvedStateRoot(ctx context.Context, stateRoot common.Hash) error
}

type FinalizedStateRootGetter interface {
	LatestFinalizedStateRoot(ctx context.Context) (common.Hash, error)
}

type StateResetter struct {
	batchResetter            BatchResetter
	finalizedStateRootGetter FinalizedStateRootGetter
	logger                   logging.Logger
}

func NewStateResetter(
	logger logging.Logger, batchResetter BatchResetter, finalizedStateRootGetter FinalizedStateRootGetter,
) *StateResetter {
	return &StateResetter{
		batchResetter:            batchResetter,
		finalizedStateRootGetter: finalizedStateRootGetter,
		logger:                   logger,
	}
}

func (r *StateResetter) ResetProgressPartial(ctx context.Context, fromBatchId scTypes.BatchId) error {
	r.logger.Info().
		Stringer(logging.FieldBatchId, fromBatchId).
		Msg("Started partial progress reset")

	purgedBatchIds, err := r.batchResetter.ResetBatchesRange(ctx, fromBatchId)
	if err != nil {
		return err
	}

	for _, batchId := range purgedBatchIds {
		// Tasks associated with the failed batch should not be cancelled at this point,
		// they will be marked as failed later
		if batchId == fromBatchId {
			continue
		}

		// todo: cancel tasks in the storage and push cancellation requests to executors
		// https://www.notion.so/nilfoundation/requires-analysis-Child-Task-Cancellation-162c61485260803882b3e50b89d2f5c4?pvs=4

		r.logger.Info().Stringer(logging.FieldBatchId, batchId).Msg("Cancelled batch tasks")
	}

	r.logger.Info().
		Stringer(logging.FieldBatchId, fromBatchId).
		Msg("Finished partial progress reset")

	return nil
}

func (r *StateResetter) ResetProgressToL1(ctx context.Context) error {
	r.logger.Info().Msg("Started all progress reset")

	if err := r.batchResetter.ResetAllBatches(ctx); err != nil {
		return fmt.Errorf("failed to reset progress for all batches: %w", err)
	}

	latestStateRoot, err := r.finalizedStateRootGetter.LatestFinalizedStateRoot(ctx)
	if err != nil {
		return fmt.Errorf("failed to get the latest finalized state root: %w", err)
	}
	if err := r.batchResetter.SetProvedStateRoot(ctx, latestStateRoot); err != nil {
		return fmt.Errorf("failed to set the latest finalized state root: %w", err)
	}

	r.logger.Info().Msg("Finished all progress reset")
	return nil
}
