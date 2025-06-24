package metrics

import (
	"fmt"

	"github.com/NilFoundation/nil/nil/internal/telemetry"
	"go.opentelemetry.io/otel/metric"
)

type ProverMetricsHandler struct {
	basicMetricsHandler
}

func NewProverMetrics() (*ProverMetricsHandler, error) {
	handler := &ProverMetricsHandler{}
	if err := initHandler("prover", handler); err != nil {
		return nil, fmt.Errorf("failed to init ProverMetricsHandler: %w", err)
	}
	return handler, nil
}

func (h *ProverMetricsHandler) init(attributes metric.MeasurementOption, meter telemetry.Meter) error {
	return h.basicMetricsHandler.init(attributes, meter)
}
