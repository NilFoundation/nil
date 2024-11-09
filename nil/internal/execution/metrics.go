package execution

import (
	"context"

	"github.com/NilFoundation/nil/nil/internal/telemetry"
	"github.com/NilFoundation/nil/nil/internal/telemetry/telattr"
	"github.com/NilFoundation/nil/nil/internal/types"
	"go.opentelemetry.io/otel/metric"
)

type MetricsHandler struct {
	option metric.MeasurementOption

	measurer *telemetry.Measurer

	// Histograms
	coinsUsedHistogram telemetry.Histogram

	// Counters
	internalMsgCounter telemetry.Counter
	externalMsgCounter telemetry.Counter

	// Gauges
	gasPrice telemetry.Gauge
	blockId  telemetry.Gauge
}

func NewMetricsHandler(name string, shardId types.ShardId) (*MetricsHandler, error) {
	meter := telemetry.NewMeter(name)
	measurer, err := telemetry.NewMeasurer(
		meter, "block_generation", telattr.ShardId(shardId),
	)
	if err != nil {
		return nil, err
	}

	handler := &MetricsHandler{
		measurer: measurer,
		option:   telattr.With(telattr.ShardId(shardId)),
	}

	if err := handler.initMetrics(meter); err != nil {
		return nil, err
	}

	return handler, nil
}

func (mh *MetricsHandler) initMetrics(meter metric.Meter) error {
	var err error

	// Initialize histograms
	mh.coinsUsedHistogram, err = meter.Int64Histogram("coins_used")
	if err != nil {
		return err
	}

	// Initialize counters
	mh.internalMsgCounter, err = meter.Int64Counter("total_blocks_processed")
	if err != nil {
		return err
	}

	mh.externalMsgCounter, err = meter.Int64Counter("total_errors_encountered")
	if err != nil {
		return err
	}

	// Initialize gauges
	mh.gasPrice, err = meter.Int64Gauge("gas_price")
	if err != nil {
		return err
	}

	mh.blockId, err = meter.Int64Gauge("block_id")
	if err != nil {
		return err
	}

	return nil
}

func (mh *MetricsHandler) StartProcessingMeasurement(ctx context.Context, gasPrice types.Value, blockId types.BlockNumber) {
	mh.measurer.Restart()
	mh.gasPrice.Record(ctx, int64(gasPrice.Uint64()), mh.option)
	mh.gasPrice.Record(ctx, int64(blockId), mh.option)
}

func (mh *MetricsHandler) EndProcessingMeasurement(ctx context.Context, internalCnt, externalCnt int64, coinsUsed types.Value) {
	mh.measurer.Measure(ctx)
	mh.internalMsgCounter.Add(ctx, internalCnt, mh.option)
	mh.externalMsgCounter.Add(ctx, externalCnt, mh.option)
	mh.coinsUsedHistogram.Record(ctx, int64(coinsUsed.Uint64()), mh.option)
}
