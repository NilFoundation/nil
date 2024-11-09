package core

import (
	"context"

	"github.com/NilFoundation/nil/nil/internal/telemetry"
	"github.com/NilFoundation/nil/nil/internal/telemetry/telattr"
	"github.com/NilFoundation/nil/nil/internal/types"
)

type MetricsHandler struct {
	measurer *telemetry.Measurer

	// Histograms
	provedBlocksHistogram telemetry.Histogram

	// Counters
	totalBlocksProcessed   telemetry.Counter
	totalErrorsEncountered telemetry.Counter
	blocksFetchedCounter   telemetry.Counter

	// Gauges
	currentBlockHeight telemetry.Gauge
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

func (mh *MetricsHandler) initMetrics(meter telemetry.Meter) error {
	var err error

	// Initialize histograms
	mh.provedBlocksHistogram, err = meter.Int64Histogram("blocks_in_tasks")
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

func (mh *MetricsHandler) StartProcessingMeasurement() {
	mh.measurer.Restart()
}

func (mh *MetricsHandler) EndProcessingMeasurement(ctx context.Context) {
	mh.measurer.Measure(ctx)
}

func (mh *MetricsHandler) RecordBlocksInTasks(ctx context.Context, count int64) {
	mh.provedBlocksHistogram.Record(ctx, count)
	mh.totalBlocksProcessed.Add(ctx, count)
}

func (mh *MetricsHandler) RecordError(ctx context.Context) {
	mh.totalErrorsEncountered.Add(ctx, 1)
}

func (mh *MetricsHandler) SetCurrentBlockHeight(ctx context.Context, height int64, shardID types.ShardId) {
	mh.currentBlockHeight.Record(ctx, height, telattr.With(telattr.ShardId(shardID)))
}
