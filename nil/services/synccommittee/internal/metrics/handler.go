package metrics

import (
	"context"
	"os"
	"time"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/telemetry"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/storage"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type BasicMetrics interface {
	RecordError(ctx context.Context, origin string)
}

type Handler struct {
	measurer   *telemetry.Measurer
	attributes metric.MeasurementOption

	totalErrorsEncountered metric.Int64Counter

	totalMainBlocksFetched metric.Int64Counter
	blockBatchSize         metric.Int64Histogram

	totalMainBlocksProved metric.Int64Counter

	totalTasksCreated     metric.Int64Counter
	totalTasksRescheduled metric.Int64Counter
	totalTasksSucceeded   metric.Int64Counter
	totalTasksFailed      metric.Int64Counter
	currentActiveTasks    metric.Int64UpDownCounter

	taskPendingTimeSeconds   metric.Float64Histogram
	taskExecutionTimeSeconds metric.Float64Histogram

	totalProposerTxSent      metric.Int64Counter
	proposalTotalTimeSeconds metric.Float64Histogram
	txPerSingleProposal      metric.Int64Histogram
}

func NewHandler(name string) (*Handler, error) {
	meter := telemetry.NewMeter(name)
	measurer, err := telemetry.NewMeasurer(meter, name)
	if err != nil {
		return nil, err
	}

	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}

	handler := &Handler{
		measurer:   measurer,
		attributes: metric.WithAttributes(attribute.String("hostname", hostname)),
	}

	if err := handler.initMetrics(name, meter); err != nil {
		return nil, err
	}

	return handler, nil
}

func (h *Handler) initMetrics(name string, meter metric.Meter) error {
	var err error

	if h.totalMainBlocksFetched, err = meter.Int64Counter(name + "_total_main_blocks_fetched"); err != nil {
		return err
	}

	if h.blockBatchSize, err = meter.Int64Histogram(name + "_block_batch_size"); err != nil {
		return err
	}

	if h.totalMainBlocksProved, err = meter.Int64Counter(name + "_total_main_blocks_proved"); err != nil {
		return err
	}

	if h.totalErrorsEncountered, err = meter.Int64Counter(name + "_total_errors_encountered"); err != nil {
		return err
	}

	if h.totalTasksCreated, err = meter.Int64Counter(name + "_total_tasks_created"); err != nil {
		return err
	}

	if h.currentActiveTasks, err = meter.Int64UpDownCounter(name + "_current_active_tasks"); err != nil {
		return err
	}

	if h.totalTasksRescheduled, err = meter.Int64Counter(name + "_total_tasks_rescheduled"); err != nil {
		return err
	}

	if h.totalTasksSucceeded, err = meter.Int64Counter(name + "_total_tasks_succeeded"); err != nil {
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

	if h.totalProposerTxSent, err = meter.Int64Counter(name + "_total_proposer_tx_sent"); err != nil {
		return err
	}

	if h.txPerSingleProposal, err = meter.Int64Histogram(name + "_tx_per_proposal"); err != nil {
		return err
	}

	return nil
}

func (h *Handler) RecordError(ctx context.Context, origin string) {
	h.totalErrorsEncountered.Add(ctx, 1, h.attributes, metric.WithAttributes(attribute.String("origin", origin)))
}

func (h *Handler) RecordMainBlockFetched(ctx context.Context, mainBlockHash common.Hash) {
	hashAttributes := metric.WithAttributes(hashAttribute(mainBlockHash))
	h.totalMainBlocksFetched.Add(ctx, 1, h.attributes, hashAttributes)
}

func (h *Handler) RecordBlockBatchSize(ctx context.Context, batchSize uint32) {
	h.blockBatchSize.Record(ctx, int64(batchSize), h.attributes)
}

func (h *Handler) RecordProposerTxSent(ctx context.Context, proposalData *storage.ProposalData) {
	proposalAttributes := []attribute.KeyValue{
		hashAttribute(proposalData.MainShardBlockHash),
		attribute.Stringer("old_proved_state_root", proposalData.OldProvedStateRoot),
		attribute.Stringer("new_proved_state_root", proposalData.NewProvedStateRoot),
	}

	h.totalProposerTxSent.Add(ctx, 1, h.attributes, metric.WithAttributes(proposalAttributes...))

	totalTimeSeconds := time.Since(proposalData.MainBlockFetchedAt).Seconds()
	h.proposalTotalTimeSeconds.Record(ctx, totalTimeSeconds, h.attributes, metric.WithAttributes(proposalAttributes...))

	txCount := int64(len(proposalData.Transactions))
	h.txPerSingleProposal.Record(ctx, txCount, h.attributes, metric.WithAttributes(proposalAttributes...))
}

func (h *Handler) RecordMainBlockProved(ctx context.Context, mainBlockHash common.Hash) {
	hashAttributes := metric.WithAttributes(hashAttribute(mainBlockHash))
	h.totalMainBlocksProved.Add(ctx, 1, h.attributes, hashAttributes)
}

func (h *Handler) RecordTaskAdded(ctx context.Context, taskEntry *types.TaskEntry) {
	taskAttributes := getTaskAttributes(taskEntry)
	h.totalTasksCreated.Add(ctx, 1, h.attributes, metric.WithAttributes(taskAttributes...))
}

func (h *Handler) RecordTaskStarted(ctx context.Context, taskEntry *types.TaskEntry) {
	taskAttributes := getTaskAttributes(taskEntry)
	pendingSeconds := taskEntry.Started.Sub(taskEntry.Created).Seconds()

	h.taskPendingTimeSeconds.Record(ctx, pendingSeconds, h.attributes, metric.WithAttributes(taskAttributes...))
	h.currentActiveTasks.Add(ctx, 1, h.attributes, metric.WithAttributes(taskAttributes...))
}

func (h *Handler) RecordTaskTerminated(ctx context.Context, taskEntry *types.TaskEntry, taskResult *types.TaskResult) {
	taskAttributes := getTaskAttributes(taskEntry)
	executionSeconds := time.Since(*taskEntry.Started).Seconds()

	h.currentActiveTasks.Add(ctx, -1, h.attributes, metric.WithAttributes(taskAttributes...))
	h.taskExecutionTimeSeconds.Record(ctx, executionSeconds, h.attributes, metric.WithAttributes(taskAttributes...))

	if taskResult.IsSuccess {
		h.totalTasksSucceeded.Add(ctx, 1, h.attributes, metric.WithAttributes(taskAttributes...))
	} else {
		h.totalTasksFailed.Add(ctx, 1, h.attributes, metric.WithAttributes(taskAttributes...))
	}
}

func (h *Handler) RecordTaskRescheduled(ctx context.Context, taskId types.TaskId) {
	h.totalTasksRescheduled.Add(ctx, 1, h.attributes, metric.WithAttributes(attribute.Stringer("task_id", taskId)))
}

func hashAttribute(mainBlockHash common.Hash) attribute.KeyValue {
	return attribute.Stringer("main_block_hash", mainBlockHash)
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
