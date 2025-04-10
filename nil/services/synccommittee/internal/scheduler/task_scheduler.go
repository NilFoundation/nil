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
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/metrics"
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

type Metrics interface {
	metrics.BasicMetrics
}

func New(
	storage Storage,
	stateHandler api.TaskStateChangeHandler,
	metrics Metrics,
	logger logging.Logger,
) TaskScheduler {
	scheduler := &taskSchedulerImpl{
		storage:      storage,
		stateHandler: stateHandler,
		config:       DefaultConfig(),
		metrics:      metrics,
	}

	scheduler.WorkerLoop = srv.NewWorkerLoop(
		"task_scheduler", scheduler.config.taskCheckInterval, scheduler.runIteration)
	scheduler.logger = srv.WorkerLogger(logger, scheduler)
	return scheduler
}

type taskSchedulerImpl struct {
	srv.WorkerLoop

	storage      Storage
	stateHandler api.TaskStateChangeHandler
	config       Config
	metrics      Metrics
	logger       logging.Logger
}

func (s *taskSchedulerImpl) runIteration(ctx context.Context) {
	err := s.storage.RescheduleHangingTasks(ctx, s.config.taskExecutionTimeout)
	if err != nil {
		s.logger.Error().Err(err).Msg("failed to reschedule hanging tasks")
		s.recordError(ctx)
	}
}

func (s *taskSchedulerImpl) GetTask(ctx context.Context, request *api.TaskRequest) (*types.Task, error) {
	s.logger.Debug().Stringer(logging.FieldTaskExecutorId, request.ExecutorId).Msg("received new task request")

	task, err := s.storage.RequestTaskToExecute(ctx, request.ExecutorId)
	if err != nil {
		s.logger.Error().
			Err(err).
			Stringer(logging.FieldTaskExecutorId, request.ExecutorId).
			Msg("failed to request task to execute")
		s.recordError(ctx)
		return nil, err
	}

	if task != nil {
		log.NewTaskEvent(s.logger, zerolog.DebugLevel, task).
			Stringer(logging.FieldTaskExecutorId, request.ExecutorId).
			Msg("task successfully requested from the storage")
	} else {
		s.logger.Debug().
			Stringer(logging.FieldTaskExecutorId, request.ExecutorId).
			Stringer(logging.FieldTaskId, nil).
			Msg("no tasks available for execution")
	}

	return task, nil
}

func (s *taskSchedulerImpl) CheckIfTaskExists(ctx context.Context, request *api.TaskCheckRequest) (bool, error) {
	s.logger.Debug().Stringer(logging.FieldTaskId, request.TaskId).Msg("received new check task request")

	taskEntry, err := s.storage.TryGetTaskEntry(ctx, request.TaskId)
	if err != nil {
		s.logger.Error().Err(err).Stringer(logging.FieldTaskId, request.TaskId).Msg("can't check if task exists")
		return false, err
	}
	if taskEntry == nil {
		s.logger.Debug().Stringer(logging.FieldTaskId, request.TaskId).Msg("task not exists")
		return false, nil
	}
	if taskEntry.Owner != request.ExecutorId {
		s.logger.Debug().Stringer(logging.FieldTaskId, request.TaskId).
			Msgf("task has unexpected executor id %d, expected %d", taskEntry.Owner, request.ExecutorId)
		return false, nil
	}

	return true, nil
}

func (s *taskSchedulerImpl) SetTaskResult(ctx context.Context, result *types.TaskResult) error {
	log.NewTaskResultEvent(s.logger, zerolog.DebugLevel, result).Msgf("received task result update")

	entry, err := s.storage.TryGetTaskEntry(ctx, result.TaskId)
	if err != nil {
		return s.onTaskResultError(ctx, err, result)
	}

	if entry == nil {
		log.NewTaskResultEvent(s.logger, zerolog.WarnLevel, result).
			Msg("received task result update for unknown task id")
		return nil
	}

	if err := result.ValidateForTask(entry); err != nil {
		return s.onTaskResultError(ctx, err, result)
	}

	if err := s.stateHandler.OnTaskTerminated(ctx, &entry.Task, result); err != nil {
		return s.onTaskResultError(ctx, err, result)
	}

	if err := s.storage.ProcessTaskResult(ctx, result); err != nil {
		return s.onTaskResultError(ctx, err, result)
	}

	return nil
}

func (s *taskSchedulerImpl) GetTasks(
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
		s.logger.Error().Err(err).Msg("failed to get tasks from the storage (GetTaskViews)")
		return nil, err
	}

	return maxHeap.PopAllSorted(), nil
}

func (s *taskSchedulerImpl) getPredicate(request *public.TaskDebugRequest) func(*public.TaskView) bool {
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

func (s *taskSchedulerImpl) getComparator(request *public.TaskDebugRequest) (func(i, j *public.TaskView) int, error) {
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

func (s *taskSchedulerImpl) GetTaskTree(ctx context.Context, taskId types.TaskId) (*public.TaskTreeView, error) {
	return s.storage.GetTaskTreeView(ctx, taskId)
}

func (s *taskSchedulerImpl) onTaskResultError(ctx context.Context, cause error, result *types.TaskResult) error {
	log.NewTaskResultEvent(s.logger, zerolog.ErrorLevel, result).Err(cause).Msg("Failed to process task result")
	s.recordError(ctx)
	return fmt.Errorf("%w: %w", ErrFailedToProcessTaskResult, cause)
}

func (s *taskSchedulerImpl) recordError(ctx context.Context) {
	s.metrics.RecordError(ctx, s.Name())
}
