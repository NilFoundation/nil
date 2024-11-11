package telemetry

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type (
	Counter       = metric.Int64Counter
	UpDownCounter = metric.Int64UpDownCounter
	Histogram     = metric.Int64Histogram
	Gauge         = metric.Int64Gauge
)

// Measurer is a helper struct to measure the duration of an operation and count the number of operations.
// It is not thread-safe.
type Measurer struct {
	counter    Counter
	histogram  Histogram
	attributes attribute.Set
	startTime  time.Time
}

func NewMeasurer(meter Meter, name string, attrs ...attribute.KeyValue) (*Measurer, error) {
	counter, err := meter.Int64Counter(name)
	if err != nil {
		return nil, err
	}
	histogram, err := meter.Int64Histogram(name + ".duration")
	if err != nil {
		return nil, err
	}
	return &Measurer{
		counter:    counter,
		histogram:  histogram,
		attributes: attribute.NewSet(attrs...),
		startTime:  time.Now(),
	}, nil
}

func (m *Measurer) Restart() {
	m.startTime = time.Now()
}

func (m *Measurer) Measure(ctx context.Context) {
	m.counter.Add(ctx, 1, metric.WithAttributeSet(m.attributes))
	m.histogram.Record(ctx, time.Since(m.startTime).Milliseconds(), metric.WithAttributeSet(m.attributes))
}
