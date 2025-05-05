package srv

import (
	"context"
	"time"

	"github.com/NilFoundation/nil/nil/common/logging"
)

type WorkerLoopAction = func(ctx context.Context) error

type WorkerLoopConfig struct {
	Name     string
	Interval time.Duration
	Action   WorkerLoopAction
}

func NewWorkerLoopConfig(name string, interval time.Duration, action WorkerLoopAction) WorkerLoopConfig {
	return WorkerLoopConfig{
		Name:     name,
		Interval: interval,
		Action:   action,
	}
}

type WorkerMetrics interface {
	RecordError(ctx context.Context, workerName string)
}

type WorkerLoop struct {
	config  WorkerLoopConfig
	metrics WorkerMetrics
	Logger  logging.Logger
}

func NewWorkerLoop(
	config WorkerLoopConfig,
	metrics WorkerMetrics,
	logger logging.Logger,
) WorkerLoop {
	loop := WorkerLoop{
		config:  config,
		metrics: metrics,
	}

	loop.Logger = WorkerLogger(logger, &loop)
	return loop
}

func (w *WorkerLoop) Name() string {
	return w.config.Name
}

func (w *WorkerLoop) Run(ctx context.Context, started chan<- struct{}) error {
	close(started)
	iteration := NewWorkerIteration(w.Logger, w.metrics, w.config.Name, w.config.Action)

	if err := iteration.Run(ctx); err != nil {
		return err
	}

	ticker := time.NewTicker(w.config.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := iteration.Run(ctx); err != nil {
				return err
			}

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
