package executor

import (
	"context"
	"crypto/rand"
	"math/big"
	"time"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/common/math"
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

func New(
	config *Config,
	requestHandler api.TaskRequestHandler,
	taskHandler api.TaskHandler,
	metrics srv.WorkerMetrics,
	logger logging.Logger,
) (*taskExecutor, error) {
	nonceId, err := generateNonceId()
	if err != nil {
		return nil, err
	}

	executor := &taskExecutor{
		nonceId:        *nonceId,
		config:         *config,
		requestHandler: requestHandler,
		taskHandler:    taskHandler,
	}

	loopConfig := srv.NewWorkerLoopConfig("task_executor", executor.config.TaskPollingInterval, executor.runIteration)
	executor.WorkerLoop = srv.NewWorkerLoop(loopConfig, metrics, logger)
	return executor, nil
}

type taskExecutor struct {
	srv.WorkerLoop

	nonceId        types.TaskExecutorId
	config         Config
	requestHandler api.TaskRequestHandler
	taskHandler    api.TaskHandler
}

func (p *taskExecutor) Id() types.TaskExecutorId {
	return p.nonceId
}

func (p *taskExecutor) runIteration(ctx context.Context) error {
	handlerReady, err := p.taskHandler.IsReadyToHandle(ctx)
	if err != nil {
		return err
	}
	if !handlerReady {
		p.Logger.Debug().Msg("handler is not ready to pick up tasks")
		return nil
	}

	taskRequest := api.NewTaskRequest(p.nonceId)
	task, err := p.requestHandler.GetTask(ctx, taskRequest)
	if err != nil {
		return err
	}

	if task == nil {
		p.Logger.Debug().Msg("no task available, waiting for new one")
		return nil
	}

	log.NewTaskEvent(p.Logger, zerolog.DebugLevel, task).Msg("Executing task")
	err = p.taskHandler.Handle(ctx, p.nonceId, task)

	if err == nil {
		log.NewTaskEvent(p.Logger, zerolog.DebugLevel, task).
			Msg("Execution of task with is successfully completed")
	} else {
		log.NewTaskEvent(p.Logger, zerolog.ErrorLevel, task).Err(err).Msg("Error handling task")
	}

	return err
}

func generateNonceId() (*types.TaskExecutorId, error) {
	bigInt, err := rand.Int(rand.Reader, big.NewInt(math.MaxInt32))
	if err != nil {
		return nil, err
	}
	nonceId := types.TaskExecutorId(uint32(bigInt.Uint64()))
	return &nonceId, nil
}
