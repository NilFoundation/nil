package srv

import (
	"context"
	"errors"
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

	if err := w.runIteration(ctx); err != nil {
		return err
	}

	ticker := time.NewTicker(w.config.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := w.runIteration(ctx); err != nil {
				return err
			}

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (w *WorkerLoop) runIteration(ctx context.Context) error {
	err := w.config.Action(ctx)
	switch {
	case err == nil:
		return nil
	case errors.Is(err, context.Canceled):
		return err
	default:
		w.Logger.Error().Err(err).Msgf("Worker %s produced an error", w.config.Name)
		w.metrics.RecordError(ctx, w.config.Name)
		return nil
	}
}
