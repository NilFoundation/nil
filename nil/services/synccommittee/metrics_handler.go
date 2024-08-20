package synccommittee

import (
	"context"

	"github.com/NilFoundation/nil/nil/internal/telemetry"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type MetricsHandler struct {
	measurer *telemetry.Measurer

	// Histograms
	provedBlocksHistogram        metric.Int64Histogram
	blockProcessingTimeHistogram metric.Float64Histogram

	// Counters
	totalBlocksProcessed   metric.Int64Counter
	totalErrorsEncountered metric.Int64Counter
	blocksFetchedCounter   metric.Int64Counter

	// Gauges
	currentBlockHeight metric.Int64Gauge
}

func NewMetricsHandler(name string) (*MetricsHandler, error) {
	meter := telemetry.NewMeter(name)
	measurer, err := telemetry.NewMeasurer(meter, "prepare_prove_tasks")
	if err != nil {
		return nil, err
	}

	handler := &MetricsHandler{
		measurer: measurer,
	}

	if err := handler.initMetrics(meter); err != nil {
		return nil, err
	}

	return handler, nil
}

func (mh *MetricsHandler) initMetrics(meter metric.Meter) error {
	var err error

	// Initialize histograms
	mh.provedBlocksHistogram, err = meter.Int64Histogram("blocks_in_tasks")
	if err != nil {
		return err
	}

	mh.blockProcessingTimeHistogram, err = meter.Float64Histogram("block_processing_time")
	if err != nil {
		return err
	}

	// Initialize counters
	mh.totalBlocksProcessed, err = meter.Int64Counter("total_blocks_processed")
	if err != nil {
		return err
	}

	mh.totalErrorsEncountered, err = meter.Int64Counter("total_errors_encountered")
	if err != nil {
		return err
	}

	mh.blocksFetchedCounter, err = meter.Int64Counter("blocks_fetched")
	if err != nil {
		return err
	}

	// Initialize gauges
	mh.currentBlockHeight, err = meter.Int64Gauge("current_block_height")
	if err != nil {
		return err
	}

	return nil
}

func (mh *MetricsHandler) RecordBlocksFetched(ctx context.Context, count int64) {
	mh.blocksFetchedCounter.Add(ctx, count)
}

func (mh *MetricsHandler) StartProcessingMeasurment() {
	mh.measurer.Restart()
}

func (mh *MetricsHandler) EndProcessingMeasurment(ctx context.Context) {
	mh.measurer.Measure(ctx)
}

func (mh *MetricsHandler) RecordBlocksInTasks(ctx context.Context, count int64) {
	mh.provedBlocksHistogram.Record(ctx, count)
	mh.totalBlocksProcessed.Add(ctx, count)
}

func (mh *MetricsHandler) RecordError(ctx context.Context) {
	mh.totalErrorsEncountered.Add(ctx, 1)
}

func (mh *MetricsHandler) SetCurrentBlockHeight(ctx context.Context, height int64, shardID uint32) {
	mh.currentBlockHeight.Record(ctx, height, metric.WithAttributes(attribute.Int64("shard_id", int64(shardID))))
}
