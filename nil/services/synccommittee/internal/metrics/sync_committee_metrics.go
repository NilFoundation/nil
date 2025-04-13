package metrics

import (
	"context"
	"fmt"
	"time"

	"github.com/NilFoundation/nil/nil/internal/telemetry"
	"github.com/NilFoundation/nil/nil/internal/telemetry/telattr"
	coreTypes "github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const (
	attrShardId = "shard.id"
)

type SyncCommitteeMetricsHandler struct {
	basicMetricsHandler
	taskStorageMetricsHandler

	attributes metric.MeasurementOption

	// AggregatorMetrics
	totalBatchesCreated telemetry.Counter
	batchSizeBlocks     telemetry.Histogram
	batchSizeTxs        telemetry.Histogram

	// LagTrackerMetrics
	blockFetchingLag telemetry.Gauge

	// BlockStorageMetrics
	totalBatchesProved telemetry.Counter

	// ProposerMetrics
	totalL1StateUpdates telemetry.Counter
	batchTotalProofTime telemetry.Histogram
}

func NewSyncCommitteeMetrics() (*SyncCommitteeMetricsHandler, error) {
	handler := &SyncCommitteeMetricsHandler{}
	if err := initHandler("sync_committee", handler); err != nil {
		return nil, fmt.Errorf("failed to init SyncCommitteeMetricsHandler: %w", err)
	}
	return handler, nil
}

func (h *SyncCommitteeMetricsHandler) init(attributes metric.MeasurementOption, meter telemetry.Meter) error {
	h.attributes = attributes

	if err := h.basicMetricsHandler.init(attributes, meter); err != nil {
		return err
	}

	if err := h.taskStorageMetricsHandler.init(attributes, meter); err != nil {
		return err
	}

	if err := h.initAggregatorMetrics(meter); err != nil {
		return err
	}

	if err := h.initLagTrackerMetrics(meter); err != nil {
		return err
	}

	if err := h.initBlockStorageMetrics(meter); err != nil {
		return err
	}

	return h.initProposerMetrics(meter)
}

func (h *SyncCommitteeMetricsHandler) initAggregatorMetrics(meter telemetry.Meter) error {
	var err error

	if h.totalBatchesCreated, err = meter.Int64Counter(namespace + "total_batches_created"); err != nil {
		return err
	}

	if h.batchSizeBlocks, err = meter.Int64Histogram(namespace + "batch_size_blocks"); err != nil {
		return err
	}

	if h.batchSizeTxs, err = meter.Int64Histogram(namespace + "batch_size_txs"); err != nil {
		return err
	}

	return nil
}

func (h *SyncCommitteeMetricsHandler) initLagTrackerMetrics(meter telemetry.Meter) error {
	var err error

	if h.blockFetchingLag, err = meter.Int64Gauge(namespace + "block_fetching_lag"); err != nil {
		return err
	}

	return nil
}

func (h *SyncCommitteeMetricsHandler) initBlockStorageMetrics(meter telemetry.Meter) error {
	var err error

	if h.totalBatchesProved, err = meter.Int64Counter(namespace + "total_batches_proved"); err != nil {
		return err
	}

	return nil
}

func (h *SyncCommitteeMetricsHandler) initProposerMetrics(meter telemetry.Meter) error {
	var err error

	if h.totalL1StateUpdates, err = meter.Int64Counter(namespace + "total_l1_state_updates"); err != nil {
		return err
	}

	if h.batchTotalProofTime, err = meter.Int64Histogram(namespace + "batch_total_proof_time_ms"); err != nil {
		return err
	}

	return nil
}

func (h *SyncCommitteeMetricsHandler) RecordBatchCreated(ctx context.Context, batch *types.BlockBatch) {
	h.totalBatchesCreated.Add(ctx, 1, h.attributes)

	var batchSizeBlocks int64
	var batchSizeTxs int64
	for block := range batch.BlocksIter() {
		batchSizeBlocks++
		batchSizeTxs += int64(len(block.Transactions))
	}

	h.batchSizeBlocks.Record(ctx, batchSizeBlocks, h.attributes)
	h.batchSizeTxs.Record(ctx, batchSizeTxs, h.attributes)
}

func (h *SyncCommitteeMetricsHandler) RecordFetchingLag(
	ctx context.Context,
	shardId coreTypes.ShardId,
	blocksCount int64,
) {
	attr := telattr.With(
		attribute.Int64(attrShardId, int64(shardId)),
	)

	h.blockFetchingLag.Record(ctx, blocksCount, h.attributes, attr)
}

func (h *SyncCommitteeMetricsHandler) RecordBatchProved(ctx context.Context) {
	h.totalBatchesProved.Add(ctx, 1, h.attributes)
}

func (h *SyncCommitteeMetricsHandler) RecordStateUpdated(ctx context.Context, proposalData *types.ProposalData) {
	h.totalL1StateUpdates.Add(ctx, 1, h.attributes)

	totalTimeMs := time.Since(proposalData.FirstBlockFetchedAt).Milliseconds()
	h.batchTotalProofTime.Record(ctx, totalTimeMs, h.attributes)
}
