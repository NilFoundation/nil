package metrics

import (
	"context"
	"fmt"

	"github.com/NilFoundation/nil/nil/internal/telemetry"
	"go.opentelemetry.io/otel/metric"
)

type FeeUpdaterMetrics struct {
	basicMetricsHandler
	l1Updates telemetry.Counter
}

func NewFeeUpdaterMetrics() (*FeeUpdaterMetrics, error) {
	handler := &FeeUpdaterMetrics{}
	if err := initHandler("fee_updater", handler); err != nil {
		return nil, fmt.Errorf("failed to init fee updater metrics: %w", err)
	}
	return handler, nil
}

func (m *FeeUpdaterMetrics) init(attributes metric.MeasurementOption, meter telemetry.Meter) error {
	m.attributes = attributes
	if err := m.basicMetricsHandler.init(attributes, meter); err != nil {
		return err
	}

	var err error
	m.l1Updates, err = meter.Int64Counter(namespace + "fee_updates_on_l1")
	if err != nil {
		return err
	}

	return nil
}

func (m *FeeUpdaterMetrics) RegisterL1Update(ctx context.Context) {
	m.l1Updates.Add(ctx, 1)
}
