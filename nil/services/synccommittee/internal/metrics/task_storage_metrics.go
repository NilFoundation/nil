package metrics

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/internal/telemetry"
	"github.com/NilFoundation/nil/nil/internal/telemetry/telattr"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const (
	attrTaskType     = "task.type"
	attrTaskExecutor = "task.executor.id"
)

type taskStorageMetricsHandler struct {
	provider atomic.Value // types.TaskStatsProvider

	attributes metric.MeasurementOption

	activeTasksByType     telemetry.ObservableUpDownCounter
	activeTasksByExecutor telemetry.ObservableUpDownCounter
	pendingTasksByType    telemetry.ObservableUpDownCounter

	totalTasksCreated     telemetry.Counter
	totalTasksSucceeded   telemetry.Counter
	totalTasksRescheduled telemetry.Counter
	totalTasksFailed      telemetry.Counter

	taskExecutionTimeMs telemetry.Histogram
}

func (h *taskStorageMetricsHandler) init(attributes metric.MeasurementOption, meter telemetry.Meter) error {
	h.attributes = attributes
	var err error
	const tasksNamespace = namespace + "tasks."

	h.activeTasksByType, err = meter.Int64ObservableUpDownCounter(tasksNamespace + "current_active_by_type")
	if err != nil {
		return err
	}

	h.activeTasksByExecutor, err = meter.Int64ObservableUpDownCounter(tasksNamespace + "current_active_by_executor")
	if err != nil {
		return err
	}

	h.pendingTasksByType, err = meter.Int64ObservableUpDownCounter(tasksNamespace + "current_pending_by_type")
	if err != nil {
		return err
	}

	if err := h.registerStatsCallback(meter); err != nil {
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

	if h.taskExecutionTimeMs, err = meter.Int64Histogram(tasksNamespace + "execution_time_ms"); err != nil {
		return err
	}

	return nil
}

func (h *taskStorageMetricsHandler) registerStatsCallback(meter telemetry.Meter) error {
	_, err := meter.RegisterCallback(
		func(ctx context.Context, observer metric.Observer) error {
			provider, ok := h.provider.Load().(types.TaskStatsProvider)
			check.PanicIfNot(ok)
			if provider == nil {
				return nil
			}

			stats, err := provider.GetTaskStats(ctx)
			if err != nil {
				return fmt.Errorf("failed to get task stats: %w", err)
			}

			for taskType, entry := range stats.CountPerType {
				attr := telattr.With(
					attribute.Stringer(attrTaskType, taskType),
				)
				observer.ObserveInt64(h.activeTasksByType, int64(entry.ActiveCount), h.attributes, attr)
				observer.ObserveInt64(h.pendingTasksByType, int64(entry.PendingCount), h.attributes, attr)
			}

			for executor, count := range stats.CountPerExecutor {
				attr := telattr.With(
					attribute.Stringer(attrTaskExecutor, executor),
				)
				observer.ObserveInt64(h.activeTasksByExecutor, int64(count), h.attributes, attr)
			}

			return nil
		},
		h.activeTasksByType, h.activeTasksByExecutor, h.pendingTasksByType,
	)
	return err
}

func (h *taskStorageMetricsHandler) SetStatsProvider(provider types.TaskStatsProvider) {
	h.provider.CompareAndSwap(nil, provider)
}

func (h *taskStorageMetricsHandler) RecordTaskAdded(ctx context.Context, taskEntry *types.TaskEntry) {
	taskAttributes := h.getAttrTypeOnly(taskEntry)
	h.totalTasksCreated.Add(ctx, 1, h.attributes, taskAttributes)
}

func (h *taskStorageMetricsHandler) RecordTaskTerminated(
	ctx context.Context,
	taskEntry *types.TaskEntry,
	taskResult *types.TaskResult,
) {
	taskAttributes := h.getAttrTypeAndOwner(taskEntry)

	if !taskResult.IsSuccess() {
		h.totalTasksFailed.Add(ctx, 1, h.attributes, taskAttributes)
		return
	}

	executionTimeMs := time.Since(*taskEntry.Started).Milliseconds()
	h.taskExecutionTimeMs.Record(ctx, executionTimeMs, h.attributes, taskAttributes)
	h.totalTasksSucceeded.Add(ctx, 1, h.attributes, taskAttributes)
}

func (h *taskStorageMetricsHandler) RecordTaskRescheduled(
	ctx context.Context,
	taskType types.TaskType,
	previousExecutor types.TaskExecutorId,
) {
	taskAttributes := telattr.With(
		attribute.Stringer(attrTaskType, taskType),
		attribute.Int64(attrTaskExecutor, int64(previousExecutor)),
	)

	h.totalTasksRescheduled.Add(ctx, 1, h.attributes, taskAttributes)
}

func (h *taskStorageMetricsHandler) getAttrTypeOnly(taskEntry *types.TaskEntry) metric.MeasurementOption {
	return telattr.With(
		attribute.Stringer(attrTaskType, taskEntry.Task.TaskType),
	)
}

func (h *taskStorageMetricsHandler) getAttrTypeAndOwner(taskEntry *types.TaskEntry) metric.MeasurementOption {
	return telattr.With(
		attribute.Stringer(attrTaskType, taskEntry.Task.TaskType),
		attribute.Int64(attrTaskExecutor, int64(taskEntry.Owner)),
	)
}
