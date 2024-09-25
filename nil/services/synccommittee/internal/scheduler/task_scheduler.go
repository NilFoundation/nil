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

func New(taskStorage storage.TaskStorage, stateHandler TaskStateChangeHandler, logger zerolog.Logger) TaskScheduler {
	return &taskSchedulerImpl{
		storage:      taskStorage,
		stateHandler: stateHandler,
		config:       DefaultConfig(),
		logger:       logger,
	}
}

type taskSchedulerImpl struct {
	storage      storage.TaskStorage
	stateHandler TaskStateChangeHandler
	config       Config
	logger       zerolog.Logger
}

func (s *taskSchedulerImpl) GetTask(ctx context.Context, request *api.TaskRequest) (*types.Task, error) {
	s.logger.Debug().Interface("executorId", request.ExecutorId).Msg("received new task request")

	task, err := s.storage.RequestTaskToExecute(ctx, request.ExecutorId)
	if err != nil {
		s.logger.Error().
			Err(err).
			Interface("executorId", request.ExecutorId).
			Msg("failed to request task to execute")
		return nil, err
	}

	return task, nil
}

func (s *taskSchedulerImpl) SetTaskResult(ctx context.Context, result *types.TaskResult) error {
	s.logger.Debug().
		Interface("executorId", result.Sender).
		Interface("taskId", result.TaskId).
		Interface("resultType", result.Type).
		Msgf("received task result update")

	entry, err := s.storage.TryGetTaskEntry(ctx, result.TaskId)
	if err != nil {
		s.logError(err, result)
		return err
	}

	if entry == nil {
		s.logger.Warn().
			Interface("executorId", result.Sender).
			Interface("taskId", result.TaskId).
			Msgf("task result update for unknown task id")
		return nil
	}

	if err = s.stateHandler.OnTaskTerminated(ctx, &entry.Task, result); err != nil {
		s.logError(err, result)
	}

	if err = s.storage.ProcessTaskResult(ctx, *result); err != nil {
		s.logError(err, result)
	}

	return err
}

func (s *taskSchedulerImpl) logError(err error, result *types.TaskResult) {
	s.logger.Error().
		Err(err).
		Interface("executorId", result.Sender).
		Interface("taskId", result.TaskId).
		Msgf("failed to process task result")
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
