package metrics

import (
	"context"
	"os"

	"github.com/NilFoundation/nil/nil/internal/telemetry"
	"github.com/NilFoundation/nil/nil/internal/types"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type MetricsHandler struct {
	measurer   *telemetry.Measurer
	attributes metric.MeasurementOption

	// Counters
	totalFromToCalls           metric.Int64Counter
	totalErrorsEncountered     metric.Int64Counter
	currentApproxWalletBalance metric.Int64UpDownCounter

	// Gauges
	currentWalletBalance metric.Int64Gauge
}

func NewMetricsHandler(name string) (*MetricsHandler, error) {
	meter := telemetry.NewMeter(name)
	measurer, err := telemetry.NewMeasurer(meter, name)
	if err != nil {
		return nil, err
	}

	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}

	handler := &MetricsHandler{
		measurer:   measurer,
		attributes: metric.WithAttributes(attribute.String("hostname", hostname)),
	}

	if err := handler.initMetrics(name, meter); err != nil {
		return nil, err
	}

	return handler, nil
}

func (mh *MetricsHandler) initMetrics(name string, meter metric.Meter) error {
	var err error
	// Initialize counters
	mh.totalErrorsEncountered, err = meter.Int64Counter(name + "_total_errors_encountered")
	if err != nil {
		return err
	}

	mh.totalFromToCalls, err = meter.Int64Counter(name + "_total_from_to_calls")
	if err != nil {
		return err
	}

	mh.currentApproxWalletBalance, err = meter.Int64UpDownCounter(name + "_current_approx_wallet_balance")
	if err != nil {
		return err
	}

	// Initialize gauges
	mh.currentWalletBalance, err = meter.Int64Gauge(name + "_current_wallet_balance")
	if err != nil {
		return err
	}

	return nil
}

func (mh *MetricsHandler) RecordFromToCall(ctx context.Context, shardFrom, shardTo int64) {
	mh.totalFromToCalls.Add(ctx, 1, mh.attributes, metric.WithAttributes(attribute.Int64("from", shardFrom), attribute.Int64("to", shardTo)))
}

func (mh *MetricsHandler) RecordError(ctx context.Context) {
	mh.totalErrorsEncountered.Add(ctx, 1, mh.attributes)
}

func (mh *MetricsHandler) SetCurrentWalletBalance(ctx context.Context, balance uint64, wallet types.Address) {
	mh.currentWalletBalance.Record(ctx, int64(balance), mh.attributes, metric.WithAttributes(attribute.String("wallet", wallet.String())))
}

func (mh *MetricsHandler) SetCurrentApproxWalletBalance(ctx context.Context, balance uint64, wallet types.Address) {
	mh.currentApproxWalletBalance.Add(ctx, int64(balance), mh.attributes, metric.WithAttributes(attribute.String("wallet", wallet.String())))
}
