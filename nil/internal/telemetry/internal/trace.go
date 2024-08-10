package internal

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func InitTracing(ctx context.Context, config *Config) error {
	if config == nil || config.TraceSamplingRate <= 0 {
		// no tracing
		return nil
	}

	var exporter sdktrace.SpanExporter
	var err error

	switch config.TraceExportOption {
	case ExportOptionNone:
		// no tracing
		return nil
	case ExportOptionStdout:
		exporter, err = newTraceStdoutExporter()
	case ExportOptionGrpc:
		exporter, err = newTraceGrpcExporter(ctx)
	default:
		return fmt.Errorf("unknown trace export option: %d", config.TraceExportOption)
	}

	if err != nil {
		return fmt.Errorf("failed to initialize exporter: %w", err)
	}

	tp, err := newTracerProvider(exporter, config)
	if err != nil {
		return fmt.Errorf("failed to initialize trace provider: %w", err)
	}

	otel.SetTracerProvider(tp)
	return nil
}

func ShutdownTracing(ctx context.Context) {
	tp, ok := otel.GetTracerProvider().(*sdktrace.TracerProvider)
	if !ok {
		// mb tracing was not initialized
		return
	}
	// nothing to do with the error
	_ = tp.Shutdown(ctx)
}

func newTraceStdoutExporter() (*stdouttrace.Exporter, error) {
	return stdouttrace.New(stdouttrace.WithPrettyPrint())
}

func newTraceGrpcExporter(ctx context.Context) (*otlptrace.Exporter, error) {
	return otlptracegrpc.New(ctx)
}

func newTracerProvider(exp sdktrace.SpanExporter, config *Config) (*sdktrace.TracerProvider, error) {
	r, err := NewResource(config)
	if err != nil {
		return nil, err
	}

	return sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithSampler(
			sdktrace.TraceIDRatioBased(config.TraceSamplingRate),
		),
		sdktrace.WithResource(r),
	), nil
}
