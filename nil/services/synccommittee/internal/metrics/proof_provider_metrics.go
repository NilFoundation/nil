package metrics

import "go.opentelemetry.io/otel/metric"

type ProofProviderMetricsHandler struct {
	basicMetricsHandler
	taskStorageMetricsHandler
}

func NewProofProviderMetrics() (*ProofProviderMetricsHandler, error) {
	handler := &ProofProviderMetricsHandler{}
	if err := initHandler("proof_provider", handler); err != nil {
		return nil, err
	}
	return handler, nil
}

func (h *ProofProviderMetricsHandler) init(name string, attributes metric.MeasurementOption, meter metric.Meter) error {
	var err error

	if err = h.basicMetricsHandler.init(name, attributes, meter); err != nil {
		return err
	}

	if err = h.taskStorageMetricsHandler.init(name, attributes, meter); err != nil {
		return err
	}

	return nil
}
