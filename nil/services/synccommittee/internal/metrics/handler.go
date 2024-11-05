package metrics

import (
	"os"

	"github.com/NilFoundation/nil/nil/internal/telemetry"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const namespace = "sync_committee."

type Handler interface {
	init(attributes metric.MeasurementOption, meter metric.Meter) error
}

func initHandler(name string, handler Handler) error {
	meter := telemetry.NewMeter(name)

	hostName, err := os.Hostname()
	if err != nil {
		return err
	}

	attributes := metric.WithAttributes(
		attribute.String("host.name", hostName),
	)

	return handler.init(attributes, meter)
}
