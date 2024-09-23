package executor

import (
	"context"
	"crypto/rand"
	"math/big"
	"time"

	"github.com/NilFoundation/nil/nil/common/concurrent"
	"github.com/NilFoundation/nil/nil/common/math"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/api"
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
	Id() types.TaskExecutorId
	Run(ctx context.Context) error
}

func New(
	config *Config,
	requestHandler api.TaskRequestHandler,
	taskHandler TaskHandler,
	logger zerolog.Logger,
) (TaskExecutor, error) {
	nonceId, err := generateNonceId()
	if err != nil {
		return nil, err
	}
	return &taskExecutorImpl{
		nonceId:        *nonceId,
		config:         *config,
		requestHandler: requestHandler,
		handler:        taskHandler,
		logger:         logger,
	}, nil
}

type taskExecutorImpl struct {
	nonceId        types.TaskExecutorId
	config         Config
	requestHandler api.TaskRequestHandler
	handler        TaskHandler
	logger         zerolog.Logger
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

	p.logger.Debug().Msgf("executing task with id=%d", task.Id)
	handleResult, err := p.handler.HandleTask(ctx, task)

	var taskResult types.TaskResult
	if err == nil {
		p.logger.Debug().Msgf("execution of task with id=%d is successfully completed", task.Id)
		taskResult = types.SuccessTaskResult(task.Id, p.nonceId, handleResult.Type, handleResult.DataAddress)
	} else {
		p.logger.Error().Err(err).Msg("error handling task")
		taskResult = types.FailureTaskResult(task.Id, p.nonceId, err)
	}

	return p.requestHandler.SetTaskResult(ctx, &taskResult)
}

func generateNonceId() (*types.TaskExecutorId, error) {
	bigInt, err := rand.Int(rand.Reader, big.NewInt(math.MaxInt32))
	if err != nil {
		return nil, err
	}
	nonceId := types.TaskExecutorId(uint32(bigInt.Uint64()))
	return &nonceId, nil
}
