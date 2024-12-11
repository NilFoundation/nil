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
	api.TaskDebugApi
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
	timer common.Timer,
	logger zerolog.Logger,
) TaskScheduler {
	return &taskSchedulerImpl{
		storage:      taskStorage,
		stateHandler: stateHandler,
		config:       DefaultConfig(),
		metrics:      metrics,
		clock:        timer,
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
		Interface("resultType", result.Type).
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

func (s *taskSchedulerImpl) GetTasks(ctx context.Context, request *api.TaskDebugRequest) ([]*types.TaskEntry, error) {
	predicate := s.getPredicate(request)
	comparator, err := s.getComparator(request)
	if err != nil {
		return nil, err
	}

	maxHeap := heap.NewBoundedMaxHeap[*types.TaskEntry](request.Limit, comparator)

	err = s.storage.GetTasks(ctx, maxHeap, predicate)
	if err != nil {
		s.logger.Error().Err(err).Msg("failed to get tasks from the storage (GetTasks)")
		return nil, err
	}

	return maxHeap.PopAllSorted(), nil
}

func (s *taskSchedulerImpl) getPredicate(request *api.TaskDebugRequest) func(entry *types.TaskEntry) bool {
	return func(entry *types.TaskEntry) bool {
		if request.Status != nil && *request.Status != entry.Status {
			return false
		}
		if request.Type != nil && *request.Type != entry.Task.TaskType {
			return false
		}
		if request.Executor != nil && *request.Executor != entry.Owner {
			return false
		}
		return true
	}
}

func (s *taskSchedulerImpl) getComparator(request *api.TaskDebugRequest) (func(i, j *types.TaskEntry) int, error) {
	var orderSign int
	if request.Ascending {
		orderSign = 1
	} else {
		orderSign = -1
	}

	currentTime := s.clock.NowTime()

	switch request.Order {
	case api.OrderByExecutionTime:
		return func(i, j *types.TaskEntry) int {
			leftExecTime := i.ExecutionTime(currentTime)
			rightExecTime := j.ExecutionTime(currentTime)
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
	case api.OrderByCreatedAt:
		return func(i, j *types.TaskEntry) int {
			switch {
			case i.Created.Before(j.Created):
				return -1 * orderSign
			case i.Created.After(j.Created):
				return orderSign
			default:
				return 0
			}
		}, nil
	default:
		return nil, fmt.Errorf("unsupported order: %s", request.Order)
	}
}

func (s *taskSchedulerImpl) GetTaskTree(ctx context.Context, taskId types.TaskId) (*types.TaskTree, error) {
	return s.storage.GetTaskTree(ctx, taskId)
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
