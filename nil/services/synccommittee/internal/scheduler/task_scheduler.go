package scheduler

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/NilFoundation/nil/nil/common/heap"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/api"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/log"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/srv"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	"github.com/NilFoundation/nil/nil/services/synccommittee/public"
	"github.com/rs/zerolog"
)

var ErrFailedToProcessTaskResult = errors.New("failed to process task result")

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
	srv.Worker
	api.TaskRequestHandler
	public.TaskDebugApi
}

type Storage interface {
	TryGetTaskEntry(ctx context.Context, id types.TaskId) (*types.TaskEntry, error)

	GetTaskViews(
		ctx context.Context,
		destination interface{ Add(task *public.TaskView) },
		predicate func(*public.TaskView) bool,
	) error

	GetTaskTreeView(ctx context.Context, taskId types.TaskId) (*public.TaskTreeView, error)

	RequestTaskToExecute(ctx context.Context, executor types.TaskExecutorId) (*types.Task, error)

	ProcessTaskResult(ctx context.Context, res *types.TaskResult) error

	RescheduleHangingTasks(ctx context.Context, taskExecutionTimeout time.Duration) error
}

func New(
	storage Storage,
	stateHandler api.TaskStateChangeHandler,
	metrics srv.WorkerMetrics,
	logger logging.Logger,
) TaskScheduler {
	scheduler := &taskScheduler{
		storage:      storage,
		stateHandler: stateHandler,
		config:       DefaultConfig(),
	}

	loopConfig := srv.NewWorkerLoopConfig("task_scheduler", scheduler.config.taskCheckInterval, scheduler.runIteration)
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
	err := s.storage.RescheduleHangingTasks(ctx, s.config.taskExecutionTimeout)
	if err == nil || errors.Is(err, context.Canceled) {
		return err
	}

	return fmt.Errorf("failed to reschedule hanging tasks: %w", err)
}

func (s *taskScheduler) GetTask(ctx context.Context, request *api.TaskRequest) (*types.Task, error) {
	s.Logger.Debug().Stringer(logging.FieldTaskExecutorId, request.ExecutorId).Msg("received new task request")

	task, err := s.storage.RequestTaskToExecute(ctx, request.ExecutorId)
	switch {
	case errors.Is(err, context.Canceled):
		return nil, err
	case err != nil:
		s.Logger.Error().
			Err(err).
			Stringer(logging.FieldTaskExecutorId, request.ExecutorId).
			Msg("failed to request task to execute")
		return nil, err
	}

	if task != nil {
		log.NewTaskEvent(s.Logger, zerolog.DebugLevel, task).
			Stringer(logging.FieldTaskExecutorId, request.ExecutorId).
			Msg("task successfully requested from the storage")
	} else {
		s.Logger.Debug().
			Stringer(logging.FieldTaskExecutorId, request.ExecutorId).
			Stringer(logging.FieldTaskId, nil).
			Msg("no tasks available for execution")
	}

	return task, nil
}

func (s *taskScheduler) CheckIfTaskExists(ctx context.Context, request *api.TaskCheckRequest) (bool, error) {
	s.Logger.Debug().Stringer(logging.FieldTaskId, request.TaskId).Msg("received new check task request")

	taskEntry, err := s.storage.TryGetTaskEntry(ctx, request.TaskId)
	if err != nil {
		s.Logger.Error().Err(err).Stringer(logging.FieldTaskId, request.TaskId).Msg("can't check if task exists")
		return false, err
	}
	if taskEntry == nil {
		s.Logger.Debug().Stringer(logging.FieldTaskId, request.TaskId).Msg("task not exists")
		return false, nil
	}
	if taskEntry.Owner != request.ExecutorId {
		s.Logger.Debug().Stringer(logging.FieldTaskId, request.TaskId).
			Msgf("task has unexpected executor id %d, expected %d", taskEntry.Owner, request.ExecutorId)
		return false, nil
	}

	return true, nil
}

func (s *taskScheduler) SetTaskResult(ctx context.Context, result *types.TaskResult) error {
	log.NewTaskResultEvent(s.Logger, zerolog.DebugLevel, result).Msgf("received task result update")

	entry, err := s.storage.TryGetTaskEntry(ctx, result.TaskId)
	if err != nil {
		return s.onTaskResultError(err, result)
	}

	if entry == nil {
		log.NewTaskResultEvent(s.Logger, zerolog.WarnLevel, result).
			Msg("received task result update for unknown task id")
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

func (s *taskScheduler) GetTasks(
	ctx context.Context,
	request *public.TaskDebugRequest,
) ([]*public.TaskView, error) {
	if err := request.Validate(); err != nil {
		return nil, err
	}

	predicate := s.getPredicate(request)
	comparator, err := s.getComparator(request)
	if err != nil {
		return nil, err
	}

	maxHeap := heap.NewBoundedMaxHeap[*public.TaskView](request.Limit, comparator)

	err = s.storage.GetTaskViews(ctx, maxHeap, predicate)
	if err != nil {
		s.Logger.Error().Err(err).Msg("failed to get tasks from the storage (GetTaskViews)")
		return nil, err
	}

	return maxHeap.PopAllSorted(), nil
}

func (s *taskScheduler) getPredicate(request *public.TaskDebugRequest) func(*public.TaskView) bool {
	return func(task *public.TaskView) bool {
		if request.Status != types.TaskStatusNone && request.Status != task.Status {
			return false
		}
		if request.Type != types.TaskTypeNone && request.Type != task.Type {
			return false
		}
		if request.Owner != types.UnknownExecutorId && request.Owner != task.Owner {
			return false
		}
		return true
	}
}

func (s *taskScheduler) getComparator(request *public.TaskDebugRequest) (func(i, j *public.TaskView) int, error) {
	var orderSign int
	if request.Ascending {
		orderSign = 1
	} else {
		orderSign = -1
	}

	switch request.Order {
	case public.OrderByExecutionTime:
		return func(i, j *public.TaskView) int {
			leftExecTime := i.ExecutionTime
			rightExecTime := j.ExecutionTime
			switch {
			case leftExecTime == nil && rightExecTime == nil:
				return 0
			case leftExecTime == nil:
				return 1
			case rightExecTime == nil:
				return -1
			case *leftExecTime < *rightExecTime:
				return -1 * orderSign
			case *leftExecTime > *rightExecTime:
				return orderSign
			default:
				return 0
			}
		}, nil
	case public.OrderByCreatedAt:
		return func(i, j *public.TaskView) int {
			switch {
			case i.CreatedAt.Before(j.CreatedAt):
				return -1 * orderSign
			case i.CreatedAt.After(j.CreatedAt):
				return orderSign
			default:
				return 0
			}
		}, nil
	default:
		return nil, fmt.Errorf("unsupported order: %s", request.Order)
	}
}

func (s *taskScheduler) GetTaskTree(ctx context.Context, taskId types.TaskId) (*public.TaskTreeView, error) {
	return s.storage.GetTaskTreeView(ctx, taskId)
}

func (s *taskScheduler) onTaskResultError(cause error, result *types.TaskResult) error {
	log.NewTaskResultEvent(s.Logger, zerolog.ErrorLevel, result).Err(cause).Msg("Failed to process task result")
	return fmt.Errorf("%w: %w", ErrFailedToProcessTaskResult, cause)
}
