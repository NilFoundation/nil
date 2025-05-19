package debug

import (
	"context"
	"fmt"

	"github.com/NilFoundation/nil/nil/common/heap"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	"github.com/NilFoundation/nil/nil/services/synccommittee/public"
)

type DebuggerStorage interface {
	GetTaskViews(
		ctx context.Context,
		destination interface{ Add(task *public.TaskView) },
		predicate func(*public.TaskView) bool,
	) error

	GetTaskTreeView(ctx context.Context, taskId types.TaskId) (*public.TaskTreeView, error)
}

type taskDebugger struct {
	storage DebuggerStorage
	logger  logging.Logger
}

func NewTaskDebugger(storage DebuggerStorage, logger logging.Logger) public.TaskDebugApi {
	return &taskDebugger{
		storage: storage,
		logger:  logger,
	}
}

func (d *taskDebugger) GetTasks(
	ctx context.Context,
	request *public.TaskDebugRequest,
) ([]*public.TaskView, error) {
	if err := request.Validate(); err != nil {
		return nil, err
	}

	predicate := d.getPredicate(request)
	comparator, err := d.getComparator(request)
	if err != nil {
		return nil, err
	}

	maxHeap := heap.NewBoundedMaxHeap[*public.TaskView](request.Limit, comparator)

	err = d.storage.GetTaskViews(ctx, maxHeap, predicate)
	if err != nil {
		d.logger.Error().Err(err).Msg("failed to get tasks from the storage (GetTaskViews)")
		return nil, err
	}

	return maxHeap.PopAllSorted(), nil
}

func (d *taskDebugger) getPredicate(request *public.TaskDebugRequest) func(*public.TaskView) bool {
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

func (d *taskDebugger) getComparator(request *public.TaskDebugRequest) (func(i, j *public.TaskView) int, error) {
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

func (d *taskDebugger) GetTaskTree(ctx context.Context, taskId types.TaskId) (*public.TaskTreeView, error) {
	return d.storage.GetTaskTreeView(ctx, taskId)
}
