package metrics

import (
	"context"
	"time"

	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type taskStorageMetricsHandler struct {
	attributes metric.MeasurementOption

	currentActiveTasks  metric.Int64UpDownCounter
	currentPendingTasks metric.Int64UpDownCounter

	totalTasksCreated     metric.Int64Counter
	totalTasksSucceeded   metric.Int64Counter
	totalTasksRescheduled metric.Int64Counter
	totalTasksFailed      metric.Int64Counter

	taskExecutionTimeSeconds metric.Float64Histogram
}

func (h *taskStorageMetricsHandler) init(attributes metric.MeasurementOption, meter metric.Meter) error {
	h.attributes = attributes
	var err error
	const tasksNamespace = namespace + "tasks."

	if h.currentActiveTasks, err = meter.Int64UpDownCounter(tasksNamespace + "current_active"); err != nil {
		return err
	}

	if h.currentPendingTasks, err = meter.Int64UpDownCounter(tasksNamespace + "current_pending"); err != nil {
		return err
	}

	if h.totalTasksCreated, err = meter.Int64Counter(tasksNamespace + "total_created"); err != nil {
		return err
	}

	if h.totalTasksSucceeded, err = meter.Int64Counter(tasksNamespace + "total_succeeded"); err != nil {
		return err
	}

	if h.totalTasksRescheduled, err = meter.Int64Counter(tasksNamespace + "total_rescheduled"); err != nil {
		return err
	}

	if h.totalTasksFailed, err = meter.Int64Counter(tasksNamespace + "total_failed"); err != nil {
		return err
	}

	if h.taskExecutionTimeSeconds, err = meter.Float64Histogram(tasksNamespace + "execution_time_seconds"); err != nil {
		return err
	}

	return nil
}

func (h *taskStorageMetricsHandler) RecordTaskAdded(ctx context.Context, taskEntry *types.TaskEntry) {
	taskAttributes := getTaskAttributes(taskEntry)
	h.totalTasksCreated.Add(ctx, 1, h.attributes, taskAttributes)
	h.currentPendingTasks.Add(ctx, 1, h.attributes, taskAttributes)
}

func (h *taskStorageMetricsHandler) RecordTaskStarted(ctx context.Context, taskEntry *types.TaskEntry) {
	taskAttributes := getTaskAttributes(taskEntry)
	h.currentPendingTasks.Add(ctx, -1, h.attributes, taskAttributes)
	h.currentActiveTasks.Add(ctx, 1, h.attributes, taskAttributes)
}

func (h *taskStorageMetricsHandler) RecordTaskTerminated(ctx context.Context, taskEntry *types.TaskEntry, taskResult *types.TaskResult) {
	taskAttributes := getTaskAttributes(taskEntry)
	h.currentActiveTasks.Add(ctx, -1, h.attributes, taskAttributes)

	if taskResult.IsSuccess {
		executionSeconds := time.Since(*taskEntry.Started).Seconds()
		h.taskExecutionTimeSeconds.Record(ctx, executionSeconds, h.attributes, taskAttributes)
		h.totalTasksSucceeded.Add(ctx, 1, h.attributes, taskAttributes)
	} else {
		h.totalTasksFailed.Add(ctx, 1, h.attributes, taskAttributes)
	}
}

func (h *taskStorageMetricsHandler) RecordTaskRescheduled(ctx context.Context, taskEntry *types.TaskEntry) {
	taskAttributes := getTaskAttributes(taskEntry)
	h.totalTasksRescheduled.Add(ctx, 1, h.attributes, taskAttributes)
	h.currentActiveTasks.Add(ctx, -1, h.attributes, taskAttributes)
	h.currentPendingTasks.Add(ctx, 1, h.attributes, taskAttributes)
}

func getTaskAttributes(task *types.TaskEntry) metric.MeasurementOption {
	attributes := []attribute.KeyValue{
		attribute.Stringer("task.type", task.Task.TaskType),
	}

	if task.Owner != types.UnknownExecutorId {
		attributes = append(attributes, attribute.Int64("task.executor.id", int64(task.Owner)))
	}

	return metric.WithAttributes(attributes...)
}
