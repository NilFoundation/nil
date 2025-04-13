package internal

import (
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0" // Be sure otelcol support it before update
)

func NewResource(config *Config) (*resource.Resource, error) {
	return resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(config.GetServiceName()),
		),
	)
}
