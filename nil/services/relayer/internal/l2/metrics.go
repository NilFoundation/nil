package l2

import (
	"context"

	"github.com/NilFoundation/nil/nil/internal/telemetry"
	"github.com/NilFoundation/nil/nil/services/relayer/internal/metrics"
	"go.opentelemetry.io/otel/metric"
)

type TransactionSenderMetrics interface {
	AddRelayedEvents(ctx context.Context, count uint64)
	AddRelayError(ctx context.Context)
}

type transactionSenderMetrics struct {
	attrs metric.MeasurementOption

	relayErrors   telemetry.Counter
	relayedEvents telemetry.Counter
}

func NewTransactionSenderMetrics() (TransactionSenderMetrics, error) {
	tsm := &transactionSenderMetrics{}
	if err := metrics.InitMetrics(tsm, "relayer", "transaction_sender"); err != nil {
		return nil, err
	}
	return tsm, nil
}

func (tsm *transactionSenderMetrics) Init(name string, meter telemetry.Meter, attrs metric.MeasurementOption) error {
	var err error

	tsm.relayedEvents, err = meter.Int64Counter(name + ".relayed_events")
	if err != nil {
		return err
	}

	tsm.relayErrors, err = meter.Int64Counter(name + ".relay_error")
	if err != nil {
		return err
	}

	tsm.attrs = attrs
	return nil
}

func (tsm *transactionSenderMetrics) AddRelayError(ctx context.Context) {
	tsm.relayErrors.Add(ctx, 1, tsm.attrs)
}

func (tsm *transactionSenderMetrics) AddRelayedEvents(ctx context.Context, count uint64) {
	tsm.relayedEvents.Add(ctx, int64(count), tsm.attrs)
}
