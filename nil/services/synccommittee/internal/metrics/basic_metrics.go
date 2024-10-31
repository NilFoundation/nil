package metrics

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type BasicMetrics interface {
	RecordError(ctx context.Context, origin string)
}

type basicMetricsHandler struct {
	attributes metric.MeasurementOption

	totalErrorsEncountered metric.Int64Counter
}

func (h *basicMetricsHandler) init(name string, attributes metric.MeasurementOption, meter metric.Meter) error {
	h.attributes = attributes
	var err error

	if h.totalErrorsEncountered, err = meter.Int64Counter(name + "_total_errors_encountered"); err != nil {
		return err
	}

	return nil
}

func (h *basicMetricsHandler) RecordError(ctx context.Context, origin string) {
	h.totalErrorsEncountered.Add(ctx, 1, h.attributes, metric.WithAttributes(attribute.String("origin", origin)))
}
