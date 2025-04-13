package telemetry

import (
	"context"

	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/internal/telemetry/internal"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

type (
	Config = internal.Config

	Meter = metric.Meter
)

func NewDefaultConfig() *Config {
	return &Config{}
}

func Init(ctx context.Context, config *Config) error {
	if config == nil {
		// no telemetry
		return nil
	}

	internal.StartPrometheusServer(config)

	return internal.InitMetrics(ctx, config)
}

func Shutdown(ctx context.Context) {
	internal.ShutdownMetrics(ctx)
}

func NewMeter(name string) Meter {
	return otel.Meter(name)
}

func Int64Gauge(meter metric.Meter, name string) metric.Int64Gauge {
	res, err := meter.Int64Gauge(name)
	check.PanicIfErr(err)
	return res
}

func Int64Counter(meter metric.Meter, name string) metric.Int64Counter {
	res, err := meter.Int64Counter(name)
	check.PanicIfErr(err)
	return res
}

func Int64UpDownCounter(meter metric.Meter, name string) metric.Int64UpDownCounter {
	res, err := meter.Int64UpDownCounter(name)
	check.PanicIfErr(err)
	return res
}
