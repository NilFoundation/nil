package telemetry

import (
	"context"

	"github.com/NilFoundation/nil/nil/internal/telemetry/internal"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

type (
	Config       = internal.Config
	ExportOption = internal.ExportOption

	Meter  = metric.Meter
	Tracer = trace.Tracer
)

const (
	ExportOptionNone   = internal.ExportOptionNone
	ExportOptionStdout = internal.ExportOptionStdout
	ExportOptionGrpc   = internal.ExportOptionGrpc
)

func Init(ctx context.Context, config *Config) error {
	if err := internal.InitTracing(ctx, config); err != nil {
		return err
	}
	if err := internal.InitMetrics(ctx, config); err != nil {
		return err
	}
	return nil
}

func Shutdown(ctx context.Context) {
	internal.ShutdownTracing(ctx)
	internal.ShutdownMetrics(ctx)
}

func NewMeter(name string) Meter {
	return otel.Meter(name)
}

func NewTracer(name string) Tracer {
	return otel.Tracer(name)
}
