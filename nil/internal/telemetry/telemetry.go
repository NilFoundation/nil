package telemetry

import (
	"context"

	"github.com/NilFoundation/nil/nil/internal/telemetry/internal"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

type (
	Config       = internal.Config
	ExportOption = internal.ExportOption

	Meter = metric.Meter
)

const (
	ExportOptionNone = internal.ExportOptionNone
	ExportOptionGrpc = internal.ExportOptionGrpc
)

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
