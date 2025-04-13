package reset

import (
	"context"
	"fmt"

	"github.com/NilFoundation/nil/nil/common/logging"
	scTypes "github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
)

type BatchResetter interface {
	// ResetBatchesRange resets Sync Committee's block processing progress
	// to a point preceding batch with the specified ID.
	ResetBatchesRange(
		ctx context.Context, firstBatchToPurge scTypes.BatchId,
	) (purgedBatches []scTypes.BatchId, err error)

	// ResetBatchesNotProved resets Sync Committee's progress for all not yet proved batches.
	ResetBatchesNotProved(ctx context.Context) error
}

type StateResetter struct {
	batchResetter BatchResetter
	logger        logging.Logger
}

func NewStateResetter(logger logging.Logger, batchResetter BatchResetter) *StateResetter {
	return &StateResetter{
		batchResetter: batchResetter,
		logger:        logger,
	}
}

func (r *StateResetter) ResetProgressPartial(ctx context.Context, failedBatchId scTypes.BatchId) error {
	r.logger.Info().
		Stringer(logging.FieldBatchId, failedBatchId).
		Msg("Started partial progress reset")

	purgedBatchIds, err := r.batchResetter.ResetBatchesRange(ctx, failedBatchId)
	if err != nil {
		return err
	}

	for _, batchId := range purgedBatchIds {
		// Tasks associated with the failed batch should not be cancelled at this point,
		// they will be marked as failed later
		if batchId == failedBatchId {
			continue
		}

		// todo: cancel tasks in the storage and push cancellation requests to executors
		// https://www.notion.so/nilfoundation/requires-analysis-Child-Task-Cancellation-162c61485260803882b3e50b89d2f5c4?pvs=4

		r.logger.Info().Stringer(logging.FieldBatchId, batchId).Msg("Cancelled batch tasks")
	}

	r.logger.Info().
		Stringer(logging.FieldBatchId, failedBatchId).
		Msg("Finished partial progress reset")

	return nil
}

func (r *StateResetter) ResetProgressNotProved(ctx context.Context) error {
	r.logger.Info().Msg("Started not proven progress reset")

	if err := r.batchResetter.ResetBatchesNotProved(ctx); err != nil {
		return fmt.Errorf("failed to reset progress for not proved batches: %w", err)
	}

	r.logger.Info().Msg("Finished not proven progress reset")
	return nil
}
