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
	deployMsgCounter   telemetry.Counter
	execMsgCounter     telemetry.Counter

	// Gauges
	gasPrice telemetry.Gauge
	blockId  telemetry.Gauge
}

type BlockGeneratorCounters struct {
	InternalMessages int64
	ExternalMessages int64
	DeployMessages   int64
	ExecMessages     int64
	CoinsUsed        types.Value
}

func NewBlockGeneratorCounters() *BlockGeneratorCounters {
	return &BlockGeneratorCounters{
		CoinsUsed: types.NewZeroValue(),
	}
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
	mh.internalMsgCounter, err = meter.Int64Counter("internal_messages_processed")
	if err != nil {
		return err
	}

	mh.externalMsgCounter, err = meter.Int64Counter("external_messages_processed")
	if err != nil {
		return err
	}

	mh.deployMsgCounter, err = meter.Int64Counter("deploy_messages_processed")
	if err != nil {
		return err
	}

	mh.execMsgCounter, err = meter.Int64Counter("execution_messages_processed")
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
	mh.blockId.Record(ctx, int64(blockId), mh.option)
}

func (mh *MetricsHandler) EndProcessingMeasurement(ctx context.Context, counters *BlockGeneratorCounters) {
	mh.measurer.Measure(ctx)
	mh.internalMsgCounter.Add(ctx, counters.InternalMessages, mh.option)
	mh.externalMsgCounter.Add(ctx, counters.ExternalMessages, mh.option)
	mh.deployMsgCounter.Add(ctx, counters.DeployMessages, mh.option)
	mh.execMsgCounter.Add(ctx, counters.ExecMessages, mh.option)
	mh.coinsUsedHistogram.Record(ctx, int64(counters.CoinsUsed.Uint64()), mh.option)
}
