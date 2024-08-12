package internal

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

const metricExportInterval = 10 * time.Second

func InitMetrics(ctx context.Context, config *Config) error {
	if config == nil {
		// no metrics
		return nil
	}

	var exporter sdkmetric.Exporter
	var err error

	switch config.MetricExportOption {
	case ExportOptionNone:
		// no metrics
		return nil
	case ExportOptionStdout:
		exporter, err = newMetricStdoutExporter()
	case ExportOptionGrpc:
		exporter, err = newMetricGrpcExporter(ctx)
	default:
		return fmt.Errorf("unknown metric export option: %d", config.MetricExportOption)
	}

	if err != nil {
		return fmt.Errorf("failed to initialize exporter: %w", err)
	}

	mp, err := newMeterProvider(exporter, config)
	if err != nil {
		return fmt.Errorf("failed to initialize metric provider: %w", err)
	}

	otel.SetMeterProvider(mp)
	return nil
}

func ShutdownMetrics(ctx context.Context) {
	mp, ok := otel.GetMeterProvider().(*sdkmetric.MeterProvider)
	if !ok {
		// mb metrics were not initialized
		return
	}
	// nothing to do with the error
	_ = mp.Shutdown(ctx)
}

func newMetricStdoutExporter() (sdkmetric.Exporter, error) {
	return stdoutmetric.New(stdoutmetric.WithPrettyPrint())
}

func newMetricGrpcExporter(ctx context.Context) (sdkmetric.Exporter, error) {
	return otlpmetricgrpc.New(ctx)
}

func newMeterProvider(exporter sdkmetric.Exporter, config *Config) (*sdkmetric.MeterProvider, error) {
	res, err := NewResource(config)
	if err != nil {
		return nil, err
	}

	return sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exporter,
			sdkmetric.WithInterval(metricExportInterval))),
		sdkmetric.WithResource(res),
	), nil
}
