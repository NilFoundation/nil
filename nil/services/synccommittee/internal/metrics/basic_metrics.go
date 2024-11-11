package metrics

import (
	"context"

	"github.com/NilFoundation/nil/nil/internal/telemetry"
	"github.com/NilFoundation/nil/nil/internal/telemetry/telattr"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type BasicMetrics interface {
	RecordError(ctx context.Context, origin string)
}

type basicMetricsHandler struct {
	attributes metric.MeasurementOption

	totalErrorsEncountered telemetry.Counter
}

func (h *basicMetricsHandler) init(attributes metric.MeasurementOption, meter telemetry.Meter) error {
	h.attributes = attributes
	var err error

	if h.totalErrorsEncountered, err = meter.Int64Counter(namespace + "total_errors_encountered"); err != nil {
		return err
	}

	return nil
}

func (h *basicMetricsHandler) RecordError(ctx context.Context, origin string) {
	errorAttributes := telattr.With(
		attribute.String("error.origin", origin),
	)

	h.totalErrorsEncountered.Add(ctx, 1, h.attributes, errorAttributes)
}
