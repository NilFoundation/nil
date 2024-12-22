package scheduler

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/concurrent"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/api"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/metrics"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/scheduler/heap"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/storage"
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
	api.TaskRequestHandler
	public.TaskDebugApi
	// Run Start task scheduler worker which monitors active tasks and reschedules them if necessary
	Run(ctx context.Context) error
}

type TaskSchedulerMetrics interface {
	metrics.BasicMetrics
}

func New(
	taskStorage storage.TaskStorage,
	stateHandler api.TaskStateChangeHandler,
	metrics TaskSchedulerMetrics,
	logger zerolog.Logger,
) TaskScheduler {
	return &taskSchedulerImpl{
		storage:      taskStorage,
		stateHandler: stateHandler,
		config:       DefaultConfig(),
		metrics:      metrics,
		clock:        common.NewTimer(),
		logger:       logger,
	}
}

type taskSchedulerImpl struct {
	storage      storage.TaskStorage
	stateHandler api.TaskStateChangeHandler
	config       Config
	metrics      TaskSchedulerMetrics
	clock        common.Timer
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
		s.recordError(ctx)
		return nil, err
	}

	if task != nil {
		s.logger.Debug().
			Interface("executorId", request.ExecutorId).
			Interface("taskId", task.Id).
			Interface("taskType", task.TaskType).
			Interface("batchId", task.BatchId).
			Interface("shardId", task.ShardId).
			Interface("blockHash", task.BlockHash).
			Msg("task successfully requested from the storage")
	} else {
		s.logger.Debug().
			Interface("executorId", request.ExecutorId).
			Interface("taskId", nil).
			Msg("no tasks available for execution")
	}

	return task, nil
}

func (s *taskSchedulerImpl) SetTaskResult(ctx context.Context, result *types.TaskResult) error {
	s.logger.Debug().
		Interface("executorId", result.Sender).
		Interface("taskId", result.TaskId).
		Msgf("received task result update")

	entry, err := s.storage.TryGetTaskEntry(ctx, result.TaskId)
	if err != nil {
		s.logError(ctx, err, result)
		return err
	}

	if entry == nil {
		s.logger.Warn().
			Interface("executorId", result.Sender).
			Interface("taskId", result.TaskId).
			Msgf("received task result update for unknown task id")
		return nil
	}

	if err = s.stateHandler.OnTaskTerminated(ctx, &entry.Task, result); err != nil {
		s.logError(ctx, err, result)
		return fmt.Errorf("%w: %w", ErrFailedToProcessTaskResult, err)
	}

	if err = s.storage.ProcessTaskResult(ctx, *result); err != nil {
		s.logError(ctx, err, result)
		return fmt.Errorf("%w: %w", ErrFailedToProcessTaskResult, err)
	}

	return nil
}

func (s *taskSchedulerImpl) GetTasks(ctx context.Context, request *public.TaskDebugRequest) ([]*public.TaskView, error) {
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

func (s *taskSchedulerImpl) logError(ctx context.Context, err error, result *types.TaskResult) {
	s.logger.
		Err(err).
		Interface("executorId", result.Sender).
		Interface("taskId", result.TaskId).
		Msg(ErrFailedToProcessTaskResult.Error())
	s.recordError(ctx)
}

func (s *taskSchedulerImpl) Run(ctx context.Context) error {
	s.logger.Info().Msg("starting task scheduler worker")

	concurrent.RunTickerLoop(ctx, s.config.taskCheckInterval, func(ctx context.Context) {
		currentTime := s.clock.NowTime()
		err := s.storage.RescheduleHangingTasks(ctx, currentTime, s.config.taskExecutionTimeout)
		if err != nil {
			s.logger.Error().Err(err).Msg("failed to reschedule hanging tasks")
			s.recordError(ctx)
		}
	})

	return nil
}

func (s *taskSchedulerImpl) recordError(ctx context.Context) {
	s.metrics.RecordError(ctx, "task_scheduler")
}
