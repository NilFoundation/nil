package metrics

import (
	"context"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	"go.opentelemetry.io/otel/metric"
)

type Handler struct {
	blockProcessingTimeHistogram metric.Float64Histogram
}

func NewHandler(name string) (*Handler, error) {
	// TODO implement me
	panic("implement me")
}

func (h *Handler) RecordMainBlockFetched(ctx context.Context, mainBlockHash common.Hash) {
	// TODO implement me
	panic("implement me")
}

func (h *Handler) RecordBlockBatchSize(ctx context.Context, batchSize int) {
	// TODO implement me
	panic("implement me")
}

func (h *Handler) RecordAggregatorError(ctx context.Context) {
	// TODO implement me
	panic("implement me")
}

func (h *Handler) RecordProposerTxSent(ctx context.Context, mainBlockHash common.Hash) {
	// TODO implement me
	panic("implement me")
}

func (h *Handler) RecordProposerError(ctx context.Context) {
	// TODO implement me
	panic("implement me")
}

func (h *Handler) RecordTaskAdded(ctx context.Context, taskId types.TaskId) {
	// TODO implement me
	panic("implement me")
}

func (h *Handler) RecordTaskStarted(ctx context.Context, taskId types.TaskId) {
	// TODO implement me
	panic("implement me")
}

func (h *Handler) RecordTaskTerminated(ctx context.Context, taskResult types.TaskResult) {
	// TODO implement me
	panic("implement me")
}

func (h *Handler) RecordTaskRescheduled(ctx context.Context, taskId types.TaskId) {
	// TODO implement me
	panic("implement me")
}
