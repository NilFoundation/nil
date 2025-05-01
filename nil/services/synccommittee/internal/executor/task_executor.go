package executor

import (
	"context"
	"time"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/api"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/log"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/srv"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	"github.com/rs/zerolog"
)

const (
	DefaultTaskPollingInterval = 10 * time.Second
)

type Config struct {
	TaskPollingInterval time.Duration
}

func DefaultConfig() *Config {
	return &Config{
		TaskPollingInterval: DefaultTaskPollingInterval,
	}
}

type IdSource interface {
	GetCurrentId(ctx context.Context) (*types.TaskExecutorId, error)
}

func New(
	config *Config,
	requestHandler api.TaskRequestHandler,
	taskHandler api.TaskHandler,
	idSource IdSource,
	metrics srv.WorkerMetrics,
	logger logging.Logger,
) (*taskExecutor, error) {
	executor := &taskExecutor{
		config:         *config,
		requestHandler: requestHandler,
		taskHandler:    taskHandler,
		idSource:       idSource,
	}

	loopConfig := srv.NewWorkerLoopConfig("task_executor", executor.config.TaskPollingInterval, executor.runIteration)
	executor.WorkerLoop = srv.NewWorkerLoop(loopConfig, metrics, logger)
	return executor, nil
}

type taskExecutor struct {
	srv.WorkerLoop

	config         Config
	requestHandler api.TaskRequestHandler
	taskHandler    api.TaskHandler
	idSource       IdSource
	logger         logging.Logger
}

func (p *taskExecutor) runIteration(ctx context.Context) error {
	handlerReady, err := p.taskHandler.IsReadyToHandle(ctx)
	if err != nil {
		return err
	}
	if !handlerReady {
		p.Logger.Debug().Msg("Handler is not ready to pick up tasks")
		return nil
	}

	executorId, err := p.idSource.GetCurrentId(ctx)
	if err != nil {
		return err
	}

	taskRequest := api.NewTaskRequest(*executorId)
	task, err := p.requestHandler.GetTask(ctx, taskRequest)
	if err != nil {
		return err
	}

	if task == nil {
		p.Logger.Debug().Msg("No task available, waiting for a new one")
		return nil
	}

	log.NewTaskEvent(p.logger, zerolog.DebugLevel, task).Msg("Executing task")
	err = p.taskHandler.Handle(ctx, *executorId, task)

	if err == nil {
		log.NewTaskEvent(p.Logger, zerolog.DebugLevel, task).
			Msg("Execution of task with is successfully completed")
	} else {
		log.NewTaskEvent(p.Logger, zerolog.ErrorLevel, task).Err(err).Msg("Error handling task")
	}

	return err
}
