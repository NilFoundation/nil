package scheduler

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/api"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/log"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/srv"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	"github.com/rs/zerolog"
)

var ErrFailedToProcessTaskResult = errors.New("failed to process task result")

type Config struct {
	TaskCheckInterval    time.Duration
	TaskExecutionTimeout time.Duration
}

func DefaultConfig() Config {
	return Config{
		TaskCheckInterval:    time.Minute,
		TaskExecutionTimeout: time.Hour,
	}
}

type Storage interface {
	TryGetTaskEntry(ctx context.Context, id types.TaskId) (*types.TaskEntry, error)

	RequestTaskToExecute(ctx context.Context, executor types.TaskExecutorId) (*types.Task, error)

	ProcessTaskResult(ctx context.Context, res *types.TaskResult) error

	RescheduleHangingTasks(ctx context.Context, taskExecutionTimeout time.Duration) error
}

func New(
	storage Storage,
	stateHandler api.TaskStateChangeHandler,
	metrics srv.WorkerMetrics,
	logger logging.Logger,
) *taskScheduler {
	scheduler := &taskScheduler{
		storage:      storage,
		stateHandler: stateHandler,
		config:       DefaultConfig(),
	}

	loopConfig := srv.NewWorkerLoopConfig("task_scheduler", scheduler.config.TaskCheckInterval, scheduler.runIteration)
	scheduler.WorkerLoop = srv.NewWorkerLoop(loopConfig, metrics, logger)
	return scheduler
}

type taskScheduler struct {
	srv.WorkerLoop

	storage      Storage
	stateHandler api.TaskStateChangeHandler
	config       Config
}

func (s *taskScheduler) runIteration(ctx context.Context) error {
	err := s.storage.RescheduleHangingTasks(ctx, s.config.TaskExecutionTimeout)
	if err == nil || errors.Is(err, context.Canceled) {
		return err
	}

	return fmt.Errorf("failed to reschedule hanging tasks: %w", err)
}

func (s *taskScheduler) GetTask(ctx context.Context, request *api.TaskRequest) (*types.Task, error) {
	s.Logger.Debug().Stringer(logging.FieldTaskExecutorId, request.ExecutorId).Msg("Received new task request")

	task, err := s.storage.RequestTaskToExecute(ctx, request.ExecutorId)
	switch {
	case errors.Is(err, context.Canceled):
		return nil, err
	case err != nil:
		s.Logger.Error().
			Err(err).
			Stringer(logging.FieldTaskExecutorId, request.ExecutorId).
			Msg("Failed to load task from the storage (RequestTaskToExecute)")
		return nil, err
	}

	if task != nil {
		log.NewTaskEvent(s.Logger, zerolog.DebugLevel, task).
			Stringer(logging.FieldTaskExecutorId, request.ExecutorId).
			Msg("Task successfully requested from the storage")
	} else {
		s.Logger.Debug().
			Stringer(logging.FieldTaskExecutorId, request.ExecutorId).
			Stringer(logging.FieldTaskId, nil).
			Msg("No tasks available for execution")
	}

	return task, nil
}

func (s *taskScheduler) CheckIfTaskIsActive(ctx context.Context, request *api.TaskCheckRequest) (bool, error) {
	s.Logger.Debug().Stringer(logging.FieldTaskId, request.TaskId).Msg("Received new check task request")

	taskEntry, err := s.storage.TryGetTaskEntry(ctx, request.TaskId)
	if err != nil {
		s.Logger.Error().
			Err(err).
			Stringer(logging.FieldTaskId, request.TaskId).
			Msg("Error during task existence check (TryGetTaskEntry)")
		return false, err
	}
	if taskEntry == nil {
		s.Logger.Debug().Stringer(logging.FieldTaskId, request.TaskId).Msg("Task with the given id does not exist")
		return false, nil
	}
	if taskEntry.Owner != request.ExecutorId {
		s.Logger.Debug().
			Stringer(logging.FieldTaskId, request.TaskId).
			Msgf("Task has unexpected executor id %d, expected %d", taskEntry.Owner, request.ExecutorId)
		return false, nil
	}

	return true, nil
}

func (s *taskScheduler) SetTaskResult(ctx context.Context, result *types.TaskResult) error {
	log.NewTaskResultEvent(s.Logger, zerolog.DebugLevel, result).Msgf("Received task result update")

	entry, err := s.storage.TryGetTaskEntry(ctx, result.TaskId)
	if err != nil {
		return s.onTaskResultError(err, result)
	}

	if entry == nil {
		log.NewTaskResultEvent(s.Logger, zerolog.WarnLevel, result).
			Msg("Received task result update for unknown task Id")
		return nil
	}

	if err := result.ValidateForTask(entry); err != nil {
		return s.onTaskResultError(err, result)
	}

	if err := s.stateHandler.OnTaskTerminated(ctx, &entry.Task, result); err != nil {
		return s.onTaskResultError(err, result)
	}

	if err := s.storage.ProcessTaskResult(ctx, result); err != nil {
		return s.onTaskResultError(err, result)
	}

	return nil
}

func (s *taskScheduler) onTaskResultError(cause error, result *types.TaskResult) error {
	log.NewTaskResultEvent(s.Logger, zerolog.ErrorLevel, result).Err(cause).Msg("Failed to process task result")
	return fmt.Errorf("%w: %w", ErrFailedToProcessTaskResult, cause)
}
