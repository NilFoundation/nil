package metrics

import (
	"os"

	"github.com/NilFoundation/nil/nil/internal/telemetry"
	"github.com/NilFoundation/nil/nil/internal/telemetry/telattr"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type Metrics interface {
	Init(name string, meter telemetry.Meter, attrs metric.MeasurementOption) error
}

func InitMetrics(mt Metrics, namespace, component string) error {
	meter := telemetry.NewMeter(namespace)

	hostName, err := os.Hostname()
	if err != nil {
		return err
	}

	attr := telattr.With(
		attribute.String("host.name", hostName),
	)

	return mt.Init(
		namespace+"."+component,
		meter,
		attr,
	)
}
