package metrics

import (
	"context"
	"fmt"
	"time"

	"github.com/NilFoundation/nil/nil/internal/telemetry"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	"go.opentelemetry.io/otel/metric"
)

type SyncCommitteeMetricsHandler struct {
	basicMetricsHandler
	taskStorageMetricsHandler

	attributes metric.MeasurementOption

	// AggregatorMetrics
	totalMainBlocksFetched telemetry.Counter
	blockBatchSize         telemetry.Histogram

	// BlockStorageMetrics
	totalBatchesProved telemetry.Counter

	// ProposerMetrics
	totalL1TxSent       telemetry.Counter
	blockTotalTimeMs    telemetry.Histogram
	txPerSingleProposal telemetry.Histogram
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

	if err := h.initBlockStorageMetrics(meter); err != nil {
		return err
	}

	if err := h.initProposerMetrics(meter); err != nil {
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

	if h.totalL1TxSent, err = meter.Int64Counter(namespace + "total_l1_tx_sent"); err != nil {
		return err
	}

	if h.blockTotalTimeMs, err = meter.Int64Histogram(namespace + "block_total_time_ms"); err != nil {
		return err
	}

	if h.txPerSingleProposal, err = meter.Int64Histogram(namespace + "tx_per_proposal"); err != nil {
		return err
	}
	return nil
}

func (h *SyncCommitteeMetricsHandler) initAggregatorMetrics(meter telemetry.Meter) error {
	var err error

	if h.totalMainBlocksFetched, err = meter.Int64Counter(namespace + "total_main_blocks_fetched"); err != nil {
		return err
	}

	if h.blockBatchSize, err = meter.Int64Histogram(namespace + "block_batch_size"); err != nil {
		return err
	}

	return nil
}

func (h *SyncCommitteeMetricsHandler) RecordMainBlockFetched(ctx context.Context) {
	h.totalMainBlocksFetched.Add(ctx, 1, h.attributes)
}

func (h *SyncCommitteeMetricsHandler) RecordBlockBatchSize(ctx context.Context, batchSize int64) {
	h.blockBatchSize.Record(ctx, batchSize, h.attributes)
}

func (h *SyncCommitteeMetricsHandler) RecordBatchProved(ctx context.Context) {
	h.totalBatchesProved.Add(ctx, 1, h.attributes)
}

func (h *SyncCommitteeMetricsHandler) RecordProposerTxSent(ctx context.Context, proposalData *types.ProposalData) {
	h.totalL1TxSent.Add(ctx, 1, h.attributes)

	totalTimeMs := time.Since(proposalData.MainBlockFetchedAt).Milliseconds()
	h.blockTotalTimeMs.Record(ctx, totalTimeMs, h.attributes)

	txCount := int64(len(proposalData.Transactions))
	h.txPerSingleProposal.Record(ctx, txCount, h.attributes)
}
