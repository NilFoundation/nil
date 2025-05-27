package batches

import (
	"context"
	"errors"
	"fmt"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/services/synccommittee/core/batches/blob"
	v1 "github.com/NilFoundation/nil/nil/services/synccommittee/core/batches/encode/v1"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/jonboulle/clockwork"
)

type RollupContract interface {
	VerifyDataProofs(ctx context.Context, commitment *Commitment) error
	CommitBatch(ctx context.Context, batchId types.BatchId, sidecar *ethtypes.BlobTxSidecar) error
}

type CommitterMetrics interface {
	RecordBatchCommitted(ctx context.Context, batch *types.BlockBatch, commitment *Commitment)
}

type committer struct {
	preparer       *commitPreparer
	rollupContract RollupContract
	clock          clockwork.Clock
	metrics        CommitterMetrics
	logger         logging.Logger
}

func NewCommitter(
	rollupContract RollupContract,
	clock clockwork.Clock,
	config CommitPreparerConfig,
	metrics CommitterMetrics,
	logger logging.Logger,
) *committer {
	encoder := v1.NewEncoder(logger)
	builder := blob.NewBuilder()
	return &committer{
		rollupContract: rollupContract,
		preparer: NewCommitPreparer(
			encoder,
			builder,
			config,
			logger,
		),
		clock:   clock,
		metrics: metrics,
		logger:  logger,
	}
}

func (c *committer) Commit(ctx context.Context, batch *types.BlockBatch) (committed *types.BlockBatch, err error) {
	if batch.IsEmpty() {
		return nil, errors.New("cannot commit empty batch")
	}

	if batch.IsSealed {
		c.logger.Info().Stringer(logging.FieldBatchId, batch.Id).Msg("Batch is already sealed, skipping commit")
		return batch, nil
	}

	commitment, err := c.preparer.PrepareBatchCommitment(batch)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare batch commitment: %w", err)
	}

	now := c.clock.Now()
	sealedBatch, err := batch.Seal(commitment.DataProofs, now)
	if err != nil {
		return nil, fmt.Errorf("failed to seal batch: %w", err)
	}

	if err := c.rollupContract.VerifyDataProofs(ctx, commitment); err != nil {
		return nil, fmt.Errorf("data proofs verification failed: %w", err)
	}

	if err := c.rollupContract.CommitBatch(ctx, sealedBatch.Id, commitment.Sidecar); err != nil {
		return nil, fmt.Errorf("failed to commit batch: %w", err)
	}

	c.metrics.RecordBatchCommitted(ctx, sealedBatch, commitment)

	return sealedBatch, nil
}
