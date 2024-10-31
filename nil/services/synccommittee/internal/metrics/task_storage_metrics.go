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

	currentActiveTasks    metric.Int64UpDownCounter
	totalTasksCreated     metric.Int64Counter
	totalTasksSucceeded   metric.Int64Counter
	totalTasksRescheduled metric.Int64Counter
	totalTasksFailed      metric.Int64Counter

	taskPendingTimeSeconds   metric.Float64Histogram
	taskExecutionTimeSeconds metric.Float64Histogram
}

func (h *taskStorageMetricsHandler) init(name string, attributes metric.MeasurementOption, meter metric.Meter) error {
	h.attributes = attributes
	var err error

	if h.currentActiveTasks, err = meter.Int64UpDownCounter(name + "_current_active_tasks"); err != nil {
		return err
	}

	if h.totalTasksCreated, err = meter.Int64Counter(name + "_total_tasks_created"); err != nil {
		return err
	}

	if h.totalTasksSucceeded, err = meter.Int64Counter(name + "_total_tasks_succeeded"); err != nil {
		return err
	}

	if h.totalTasksRescheduled, err = meter.Int64Counter(name + "_total_tasks_rescheduled"); err != nil {
		return err
	}

	if h.totalTasksFailed, err = meter.Int64Counter(name + "_total_tasks_failed"); err != nil {
		return err
	}

	if h.taskPendingTimeSeconds, err = meter.Float64Histogram(name + "_task_pending_time_seconds"); err != nil {
		return err
	}

	if h.taskExecutionTimeSeconds, err = meter.Float64Histogram(name + "_task_execution_time_seconds"); err != nil {
		return err
	}

	return nil
}

func (h *taskStorageMetricsHandler) RecordTaskAdded(ctx context.Context, taskEntry *types.TaskEntry) {
	taskAttributes := getTaskAttributes(taskEntry)
	h.totalTasksCreated.Add(ctx, 1, h.attributes, metric.WithAttributes(taskAttributes...))
}

func (h *taskStorageMetricsHandler) RecordTaskStarted(ctx context.Context, taskEntry *types.TaskEntry) {
	taskAttributes := getTaskAttributes(taskEntry)
	pendingSeconds := taskEntry.Started.Sub(taskEntry.Created).Seconds()

	h.taskPendingTimeSeconds.Record(ctx, pendingSeconds, h.attributes, metric.WithAttributes(taskAttributes...))
	h.currentActiveTasks.Add(ctx, 1, h.attributes, metric.WithAttributes(taskAttributes...))
}

func (h *taskStorageMetricsHandler) RecordTaskTerminated(ctx context.Context, taskEntry *types.TaskEntry, taskResult *types.TaskResult) {
	taskAttributes := getTaskAttributes(taskEntry)
	executionSeconds := time.Since(*taskEntry.Started).Seconds()

	h.currentActiveTasks.Add(ctx, -1, h.attributes, metric.WithAttributes(taskAttributes...))
	h.taskExecutionTimeSeconds.Record(ctx, executionSeconds, h.attributes, metric.WithAttributes(taskAttributes...))

	if taskResult.IsSuccess {
		h.totalTasksSucceeded.Add(ctx, 1, h.attributes, metric.WithAttributes(taskAttributes...))
	} else {
		taskAttributes = append(taskAttributes, attribute.String("error_text", taskResult.ErrorText))
		h.totalTasksFailed.Add(ctx, 1, h.attributes, metric.WithAttributes(taskAttributes...))
	}
}

func (h *taskStorageMetricsHandler) RecordTaskRescheduled(ctx context.Context, taskId types.TaskId) {
	h.totalTasksRescheduled.Add(ctx, 1, h.attributes, metric.WithAttributes(attribute.Stringer("task_id", taskId)))
}

func getTaskAttributes(task *types.TaskEntry) []attribute.KeyValue {
	attributes := []attribute.KeyValue{
		attribute.Stringer("task_id", task.Task.Id),
		attribute.Stringer("task_type", task.Task.TaskType),
	}

	if task.Owner != types.UnknownExecutorId {
		attributes = append(attributes, attribute.Int64("executor_id", int64(task.Owner)))
	}

	return attributes
}
