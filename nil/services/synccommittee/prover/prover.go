package prover

import (
	"context"
	"crypto/rand"
	"math/big"
	"time"

	"github.com/NilFoundation/nil/nil/common/concurrent"
	"github.com/NilFoundation/nil/nil/common/math"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/api"
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

type Prover struct {
	nonceId  api.ProverNonceId
	config   Config
	observer api.TaskObserver
	handler  TaskHandler
	logger   zerolog.Logger
}

func NewProver(config *Config, observer api.TaskObserver, handler TaskHandler, logger zerolog.Logger) (*Prover, error) {
	nonceId, err := generateProverNonceId()
	if err != nil {
		return nil, err
	}
	return &Prover{
		nonceId:  *nonceId,
		config:   *config,
		observer: observer,
		handler:  handler,
		logger:   logger,
	}, nil
}

func (p *Prover) Run(ctx context.Context) error {
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

func (p *Prover) fetchAndHandleTask(ctx context.Context) error {
	taskRequest := api.NewTaskRequest(p.nonceId)
	task, err := p.observer.GetTask(ctx, taskRequest)
	if err != nil {
		return err
	}

	if task == nil {
		p.logger.Debug().Msg("no task available, waiting for new one")
		return nil
	}

	p.logger.Debug().Msgf("executing task with id=%d", task.Id)
	handleError := p.handler.HandleTask(ctx, task)

	var taskStatus api.ProverTaskStatus
	if handleError == nil {
		p.logger.Debug().Msgf("execution of task with id=%d is successfully completed", task.Id)
		taskStatus = api.Done
	} else {
		p.logger.Error().Err(handleError).Msg("error handling task")
		taskStatus = api.Failed
	}

	statusUpdateRequest := api.NewTaskStatusUpdateRequest(p.nonceId, task.Id, taskStatus)
	return p.observer.UpdateTaskStatus(ctx, statusUpdateRequest)
}

func generateProverNonceId() (*api.ProverNonceId, error) {
	bigInt, err := rand.Int(rand.Reader, big.NewInt(math.MaxInt32))
	if err != nil {
		return nil, err
	}
	nonceId := api.ProverNonceId(uint32(bigInt.Uint64()))
	return &nonceId, nil
}
