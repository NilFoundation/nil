package metrics

import (
	"context"

	"github.com/NilFoundation/nil/nil/internal/telemetry"
	"github.com/NilFoundation/nil/nil/services/relayer/internal/metrics"
	"go.opentelemetry.io/otel/metric"
)

type heartbeatMetricHandler struct {
	attributes metric.MeasurementOption

	heartbeat telemetry.Counter
}

func NewHeartbeatMetricHandler() (*heartbeatMetricHandler, error) {
	hb := &heartbeatMetricHandler{}
	if err := metrics.InitMetrics(hb, "relayer", "heartbeat"); err != nil {
		return nil, err
	}
	return hb, nil
}

func (h *heartbeatMetricHandler) Init(name string, meter telemetry.Meter, attrs metric.MeasurementOption) error {
	h.attributes = attrs
	var err error

	if h.heartbeat, err = meter.Int64Counter(name + ".heartbeat"); err != nil {
		return err
	}

	return nil
}

func (h *heartbeatMetricHandler) Heartbeat(ctx context.Context) {
	h.heartbeat.Add(ctx, 1, h.attributes)
}
