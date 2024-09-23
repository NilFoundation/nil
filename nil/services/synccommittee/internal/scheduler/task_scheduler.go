package scheduler

import (
	"context"
	"time"

	"github.com/NilFoundation/nil/nil/common/concurrent"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/api"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/storage"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	"github.com/rs/zerolog"
)

type Config struct {
	taskCheckInterval    time.Duration
	taskExecutionTimeout time.Duration
}

func DefaultConfig() Config {
	return Config{
		taskCheckInterval:    time.Minute,
		taskExecutionTimeout: time.Hour,
	}
}

type TaskScheduler interface {
	api.TaskRequestHandler
	// Run Start task scheduler worker which monitors active tasks and reschedules them if necessary
	Run(ctx context.Context) error
}

func NewTaskScheduler(taskStorage storage.ProverTaskStorage, logger zerolog.Logger) TaskScheduler {
	return &taskSchedulerImpl{
		storage: taskStorage,
		config:  DefaultConfig(),
		logger:  logger,
	}
}

type taskSchedulerImpl struct {
	storage storage.ProverTaskStorage
	config  Config
	logger  zerolog.Logger
}

func (s *taskSchedulerImpl) GetTask(ctx context.Context, request *api.TaskRequest) (*types.ProverTask, error) {
	s.logger.Debug().Interface("proverId", request.ExecutorId).Msg("received new task request")

	task, err := s.storage.RequestTaskToExecute(ctx, request.ExecutorId)
	if err != nil {
		s.logger.Error().
			Err(err).
			Interface("proverId", request.ExecutorId).
			Msg("failed to request task to execute")
		return nil, err
	}

	return task, nil
}

func (s *taskSchedulerImpl) SetTaskResult(ctx context.Context, result *types.TaskResult) error {
	s.logger.Debug().
		Interface("proverId", result.Sender).
		Interface("taskId", result.TaskId).
		Interface("resultType", result.Type).
		Msgf("received task result update")

	err := s.storage.ProcessTaskResult(ctx, *result)
	if err != nil {
		s.logger.Error().
			Err(err).
			Interface("proverId", result.Sender).
			Interface("taskId", result.TaskId).
			Msgf("failed to process task result")
	}

	return err
}

func (s *taskSchedulerImpl) Run(ctx context.Context) error {
	s.logger.Info().Msg("starting task scheduler worker")

	concurrent.RunTickerLoop(ctx, s.config.taskCheckInterval, func(ctx context.Context) {
		currentTime := time.Now()
		err := s.storage.RescheduleHangingTasks(ctx, currentTime, s.config.taskExecutionTimeout)
		if err != nil {
			s.logger.Error().Err(err).Msg("failed to reschedule hanging tasks")
		}
	})

	return nil
}
