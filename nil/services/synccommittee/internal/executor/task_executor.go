package executor

import (
	"context"
	"crypto/rand"
	"math/big"
	"time"

	"github.com/NilFoundation/nil/nil/common/concurrent"
	"github.com/NilFoundation/nil/nil/common/math"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/api"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/log"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/metrics"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/srv"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	"github.com/rs/zerolog"
)

const (
	DefaultTaskPollingInterval = time.Second
)

type Config struct {
	TaskPollingInterval time.Duration
}

func DefaultConfig() *Config {
	return &Config{
		TaskPollingInterval: DefaultTaskPollingInterval,
	}
}

type TaskExecutor interface {
	srv.Worker
	Id() types.TaskExecutorId
}

type TaskExecutorMetrics interface {
	metrics.BasicMetrics
}

func New(
	config *Config,
	requestHandler api.TaskRequestHandler,
	taskHandler api.TaskHandler,
	metrics TaskExecutorMetrics,
	logger zerolog.Logger,
) (TaskExecutor, error) {
	nonceId, err := generateNonceId()
	if err != nil {
		return nil, err
	}

	executor := &taskExecutorImpl{
		nonceId:        *nonceId,
		config:         *config,
		requestHandler: requestHandler,
		taskHandler:    taskHandler,
		metrics:        metrics,
	}

	executor.logger = srv.LoggerWithWorkerName(logger, executor.Name())
	return executor, nil
}

type taskExecutorImpl struct {
	nonceId        types.TaskExecutorId
	config         Config
	requestHandler api.TaskRequestHandler
	taskHandler    api.TaskHandler
	metrics        TaskExecutorMetrics
	logger         zerolog.Logger
}

func (*taskExecutorImpl) Name() string {
	return "task_executor"
}

func (p *taskExecutorImpl) Id() types.TaskExecutorId {
	return p.nonceId
}

func (p *taskExecutorImpl) Run(ctx context.Context) error {
	concurrent.RunTickerLoop(
		ctx,
		p.config.TaskPollingInterval,
		func(ctx context.Context) {
			if err := p.fetchAndHandleTask(ctx); err != nil {
				p.logger.Error().Err(err).Msg("failed to fetch and handle next task")
				p.metrics.RecordError(ctx, p.Name())
			}
		},
	)

	return nil
}

func (p *taskExecutorImpl) fetchAndHandleTask(ctx context.Context) error {
	taskRequest := api.NewTaskRequest(p.nonceId)
	task, err := p.requestHandler.GetTask(ctx, taskRequest)
	if err != nil {
		return err
	}

	if task == nil {
		p.logger.Debug().Msg("no task available, waiting for new one")
		return nil
	}

	log.NewTaskEvent(p.logger, zerolog.DebugLevel, task).Msg("Executing task")
	err = p.taskHandler.Handle(ctx, p.nonceId, task)

	if err == nil {
		log.NewTaskEvent(p.logger, zerolog.DebugLevel, task).Msg("Execution of task with is successfully completed")
	} else {
		log.NewTaskEvent(p.logger, zerolog.ErrorLevel, task).Err(err).Msg("Error handling task")
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
