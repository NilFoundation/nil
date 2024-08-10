package telemetry

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/metric"
)

type (
	Counter   = metric.Int64Counter
	Histogram = metric.Int64Histogram
)

// Measurer is a helper struct to measure the duration of an operation and count the number of operations.
// It is not thread-safe.
type Measurer struct {
	counter   Counter
	histogram Histogram
	startTime time.Time
}

func NewMeasurer(meter Meter, name string) (*Measurer, error) {
	counter, err := meter.Int64Counter(name)
	if err != nil {
		return nil, err
	}
	histogram, err := meter.Int64Histogram(name + ".duration")
	if err != nil {
		return nil, err
	}
	return &Measurer{
		counter:   counter,
		histogram: histogram,
		startTime: time.Now(),
	}, nil
}

func (m *Measurer) Restart() {
	m.startTime = time.Now()
}

func (m *Measurer) Measure(ctx context.Context) {
	m.counter.Add(ctx, 1)
	m.histogram.Record(ctx, time.Since(m.startTime).Milliseconds())
}
