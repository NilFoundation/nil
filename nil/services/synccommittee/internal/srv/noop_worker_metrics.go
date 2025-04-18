//go:build test

package srv

import "context"

type noopMetrics struct{}

func NewNoopWorkerMetrics() WorkerMetrics {
	return &noopMetrics{}
}

func (m *noopMetrics) Heartbeat(_ context.Context) {}

func (m *noopMetrics) RecordError(_ context.Context, _ string) {}
