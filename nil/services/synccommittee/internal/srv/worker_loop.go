package srv

import (
	"context"
	"time"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/jonboulle/clockwork"
)

type (
	WorkerLoopAction = func(ctx context.Context) error
	StarterAction    = func(ctx context.Context) error
)

type WorkerLoopConfig struct {
	Name     string
	Interval time.Duration
	Action   WorkerLoopAction
	Starter  StarterAction
	Clock    clockwork.Clock
}

func NewWorkerLoopConfig(
	name string,
	interval time.Duration,
	action WorkerLoopAction,
	options ...WorkerOption,
) WorkerLoopConfig {
	cfg := WorkerLoopConfig{
		Name:     name,
		Interval: interval,
		Action:   action,
		Clock:    clockwork.NewRealClock(), // Default clock
	}
	for _, option := range options {
		option(&cfg)
	}
	return cfg
}

type WorkerOption func(*WorkerLoopConfig)

func WithStarter(starter StarterAction) WorkerOption {
	return func(cfg *WorkerLoopConfig) {
		cfg.Starter = starter
	}
}

func WithClock(clock clockwork.Clock) WorkerOption {
	return func(cfg *WorkerLoopConfig) {
		cfg.Clock = clock
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
	if w.config.Starter != nil {
		if err := w.config.Starter(ctx); err != nil {
			return err
		}
	}

	ticker := w.config.Clock.NewTicker(w.config.Interval)
	defer ticker.Stop()

	close(started)

	iteration := NewWorkerIteration(w.Logger, w.metrics, w.config.Name, w.config.Action)

	if err := iteration.Run(ctx); err != nil {
		return err
	}

	for {
		select {
		case <-ticker.Chan():
			if err := iteration.Run(ctx); err != nil {
				return err
			}

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
