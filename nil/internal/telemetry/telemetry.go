package telemetry

import (
	"context"
	"os"

	"github.com/NilFoundation/nil/nil/internal/telemetry/internal"
	"github.com/spf13/pflag"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

type (
	Config = internal.Config

	Meter = metric.Meter
)

func NewDefaultConfig() *Config {
	return &Config{
		ServiceName: os.Args[0],
	}
}

func AddFlags(fset *pflag.FlagSet, config *Config) {
	fset.BoolVar(&config.ExportMetrics, "metrics", config.ExportMetrics, "export metrics via grpc")
}

func Init(ctx context.Context, config *Config) error {
	if err := internal.InitMetrics(ctx, config); err != nil {
		return err
	}
	return nil
}

func Shutdown(ctx context.Context) {
	internal.ShutdownMetrics(ctx)
}

func NewMeter(name string) Meter {
	return otel.Meter(name)
}
