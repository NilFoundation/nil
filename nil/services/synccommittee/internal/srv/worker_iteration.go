package srv

import (
	"context"
	"errors"

	"github.com/NilFoundation/nil/nil/common/logging"
)

type workerIteration struct {
	logger  logging.Logger
	metrics WorkerMetrics
	name    string
	action  func(context.Context) error
}

func NewWorkerIteration(
	logger logging.Logger,
	metrics WorkerMetrics,
	name string,
	action WorkerLoopAction,
) workerIteration {
	return workerIteration{
		logger:  logger,
		metrics: metrics,
		name:    name,
		action:  action,
	}
}

func (i *workerIteration) Run(ctx context.Context) error {
	err := i.action(ctx)
	switch {
	case err == nil:
		return nil
	case errors.Is(err, context.Canceled):
		return err
	default:
		i.logger.Error().Err(err).Str(logging.FieldWorkerName, i.name).Msgf("Worker %s produced an error", i.name)
		i.metrics.RecordError(ctx, i.name)
		return nil
	}
}
