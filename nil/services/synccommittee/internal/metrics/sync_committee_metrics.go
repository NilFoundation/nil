package metrics

import (
	"context"
	"time"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	"go.opentelemetry.io/otel/attribute"
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
		return nil, err
	}
	return handler, nil
}

func (h *SyncCommitteeMetricsHandler) init(name string, attributes metric.MeasurementOption, meter metric.Meter) error {
	h.attributes = attributes

	if err := h.basicMetricsHandler.init(name, attributes, meter); err != nil {
		return err
	}

	if err := h.taskStorageMetricsHandler.init(name, attributes, meter); err != nil {
		return err
	}

	if err := h.initAggregatorMetrics(name, meter); err != nil {
		return err
	}

	if err := h.initBlockStorageMetrics(name, meter); err != nil {
		return err
	}

	if err := h.initProposerMetrics(name, meter); err != nil {
		return err
	}

	return nil
}

func (h *SyncCommitteeMetricsHandler) initBlockStorageMetrics(name string, meter metric.Meter) error {
	var err error

	if h.totalMainBlocksProved, err = meter.Int64Counter(name + "_total_main_blocks_proved"); err != nil {
		return err
	}

	return nil
}

func (h *SyncCommitteeMetricsHandler) initProposerMetrics(name string, meter metric.Meter) error {
	var err error

	if h.totalL1TxSent, err = meter.Int64Counter(name + "_total_proposer_l1_tx_sent"); err != nil {
		return err
	}

	if h.blockTotalTimeSeconds, err = meter.Float64Histogram(name + "_block_total_time_seconds"); err != nil {
		return err
	}

	if h.txPerSingleProposal, err = meter.Int64Histogram(name + "_tx_per_proposal"); err != nil {
		return err
	}
	return nil
}

func (h *SyncCommitteeMetricsHandler) initAggregatorMetrics(name string, meter metric.Meter) error {
	var err error

	if h.totalMainBlocksFetched, err = meter.Int64Counter(name + "_total_main_blocks_fetched"); err != nil {
		return err
	}

	if h.blockBatchSize, err = meter.Int64Histogram(name + "_block_batch_size"); err != nil {
		return err
	}

	return nil
}

func (h *SyncCommitteeMetricsHandler) RecordMainBlockFetched(ctx context.Context, mainBlockHash common.Hash) {
	hashAttributes := metric.WithAttributes(hashAttribute(mainBlockHash))
	h.totalMainBlocksFetched.Add(ctx, 1, h.attributes, hashAttributes)
}

func (h *SyncCommitteeMetricsHandler) RecordBlockBatchSize(ctx context.Context, batchSize uint32) {
	h.blockBatchSize.Record(ctx, int64(batchSize), h.attributes)
}

func (h *SyncCommitteeMetricsHandler) RecordMainBlockProved(ctx context.Context, mainBlockHash common.Hash) {
	hashAttributes := metric.WithAttributes(hashAttribute(mainBlockHash))
	h.totalMainBlocksProved.Add(ctx, 1, h.attributes, hashAttributes)
}

func (h *SyncCommitteeMetricsHandler) RecordProposerTxSent(ctx context.Context, proposalData *types.ProposalData) {
	proposalAttributes := []attribute.KeyValue{
		hashAttribute(proposalData.MainShardBlockHash),
		attribute.Stringer("old_proved_state_root", proposalData.OldProvedStateRoot),
		attribute.Stringer("new_proved_state_root", proposalData.NewProvedStateRoot),
	}

	h.totalL1TxSent.Add(ctx, 1, h.attributes, metric.WithAttributes(proposalAttributes...))

	totalTimeSeconds := time.Since(proposalData.MainBlockFetchedAt).Seconds()
	h.blockTotalTimeSeconds.Record(ctx, totalTimeSeconds, h.attributes, metric.WithAttributes(proposalAttributes...))

	txCount := int64(len(proposalData.Transactions))
	h.txPerSingleProposal.Record(ctx, txCount, h.attributes, metric.WithAttributes(proposalAttributes...))
}

func hashAttribute(mainBlockHash common.Hash) attribute.KeyValue {
	return attribute.Stringer("main_block_hash", mainBlockHash)
}
