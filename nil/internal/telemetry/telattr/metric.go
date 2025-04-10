package telattr

import (
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type MetricOption = metric.MeasurementOption

func With(attrs ...attribute.KeyValue) MetricOption {
	return metric.WithAttributeSet(attribute.NewSet(attrs...))
}
