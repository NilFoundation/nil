package metrics

import (
	"os"

	"github.com/NilFoundation/nil/nil/internal/telemetry"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type Handler interface {
	init(name string, attributes metric.MeasurementOption, meter metric.Meter) error
}

func initHandler(name string, handler Handler) error {
	meter := telemetry.NewMeter(name)

	hostname, err := os.Hostname()
	if err != nil {
		return err
	}

	attributes := metric.WithAttributes(attribute.String("hostname", hostname))
	return handler.init(name, attributes, meter)
}
