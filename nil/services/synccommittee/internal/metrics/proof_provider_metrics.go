package metrics

import (
	"fmt"

	"github.com/NilFoundation/nil/nil/internal/telemetry"
	"go.opentelemetry.io/otel/metric"
)

type ProofProviderMetricsHandler struct {
	basicMetricsHandler
	taskStorageMetricsHandler
}

func NewProofProviderMetrics() (*ProofProviderMetricsHandler, error) {
	handler := &ProofProviderMetricsHandler{}
	if err := initHandler("proof_provider", handler); err != nil {
		return nil, fmt.Errorf("failed to init ProofProviderMetricsHandler: %w", err)
	}
	return handler, nil
}

func (h *ProofProviderMetricsHandler) init(attributes metric.MeasurementOption, meter telemetry.Meter) error {
	var err error

	if err = h.basicMetricsHandler.init(attributes, meter); err != nil {
		return err
	}

	if err = h.taskStorageMetricsHandler.init(attributes, meter); err != nil {
		return err
	}

	return nil
}
