package metrics

import (
	"context"
	"fmt"
	"time"

	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	"go.opentelemetry.io/otel/metric"
)

type SyncCommitteeMetricsHandler struct {
	basicMetricsHandler
	taskStorageMetricsHandler

	attributes metric.MeasurementOption

	// AggregatorMetrics
	totalMainBlocksFetched metric.Int64Counter
	blockBatchSize         metric.Int64Histogram

	// BlockStorageMetrics
	totalMainBlocksProved metric.Int64Counter

	// ProposerMetrics
	totalL1TxSent         metric.Int64Counter
	blockTotalTimeSeconds metric.Float64Histogram
	txPerSingleProposal   metric.Int64Histogram
}

func NewSyncCommitteeMetrics() (*SyncCommitteeMetricsHandler, error) {
	handler := &SyncCommitteeMetricsHandler{}
	if err := initHandler("sync_committee", handler); err != nil {
		return nil, fmt.Errorf("failed to init SyncCommitteeMetricsHandler: %w", err)
	}
	return handler, nil
}

func (h *SyncCommitteeMetricsHandler) init(attributes metric.MeasurementOption, meter metric.Meter) error {
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

func (h *SyncCommitteeMetricsHandler) initBlockStorageMetrics(meter metric.Meter) error {
	var err error

	if h.totalMainBlocksProved, err = meter.Int64Counter(namespace + "total_main_blocks_proved"); err != nil {
		return err
	}

	return nil
}

func (h *SyncCommitteeMetricsHandler) initProposerMetrics(meter metric.Meter) error {
	var err error

	if h.totalL1TxSent, err = meter.Int64Counter(namespace + "total_l1_tx_sent"); err != nil {
		return err
	}

	if h.blockTotalTimeSeconds, err = meter.Float64Histogram(namespace + "block_total_time_seconds"); err != nil {
		return err
	}

	if h.txPerSingleProposal, err = meter.Int64Histogram(namespace + "tx_per_proposal"); err != nil {
		return err
	}
	return nil
}

func (h *SyncCommitteeMetricsHandler) initAggregatorMetrics(meter metric.Meter) error {
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

func (h *SyncCommitteeMetricsHandler) RecordBlockBatchSize(ctx context.Context, batchSize uint32) {
	h.blockBatchSize.Record(ctx, int64(batchSize), h.attributes)
}

func (h *SyncCommitteeMetricsHandler) RecordMainBlockProved(ctx context.Context) {
	h.totalMainBlocksProved.Add(ctx, 1, h.attributes)
}

func (h *SyncCommitteeMetricsHandler) RecordProposerTxSent(ctx context.Context, proposalData *types.ProposalData) {
	h.totalL1TxSent.Add(ctx, 1, h.attributes)

	totalTimeSeconds := time.Since(proposalData.MainBlockFetchedAt).Seconds()
	h.blockTotalTimeSeconds.Record(ctx, totalTimeSeconds, h.attributes)

	txCount := int64(len(proposalData.Transactions))
	h.txPerSingleProposal.Record(ctx, txCount, h.attributes)
}
