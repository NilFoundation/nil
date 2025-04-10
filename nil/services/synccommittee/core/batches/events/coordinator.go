package events

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/metrics"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/srv"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
)

type CoordinatorMetrics interface {
	metrics.BasicMetrics
}

type BatchEventStorage interface {
	TryGetPendingEvent(ctx context.Context) (*types.BatchEvent, *types.BlockBatch, error)
	OnBatchEventSkipped(ctx context.Context, eventId types.BatchEventId) error
	OnBatchEventHandled(ctx context.Context, eventId types.BatchEventId, batch *types.BlockBatch) error
}

type batchCoordinator struct {
	srv.WorkerLoop

	handlers map[types.BatchStatus]BatchStatusChangeHandler
	storage  BatchEventStorage
	metrics  CoordinatorMetrics
	logger   logging.Logger
}

func NewBatchCoordinator(
	handlers []BatchStatusChangeHandler,
	storage BatchEventStorage,
	metrics CoordinatorMetrics,
	logger logging.Logger,
) *batchCoordinator {
	mappedHandlers := make(map[types.BatchStatus]BatchStatusChangeHandler)
	for _, handler := range handlers {
		for status := range handler.SupportedStatuses() {
			check.PanicIff(mappedHandlers[status] != nil, "duplicate handler for status %s", status)
			mappedHandlers[status] = handler
		}
	}

	coordinator := &batchCoordinator{
		handlers: mappedHandlers,
		storage:  storage,
		metrics:  metrics,
		logger:   logger,
	}

	const eventPollingInterval = time.Second
	coordinator.WorkerLoop = srv.NewWorkerLoop("batch_coordinator", eventPollingInterval, coordinator.runIteration)
	return coordinator
}

func (c *batchCoordinator) runIteration(ctx context.Context) {
	err := c.processPendingEvent(ctx)

	if err == nil || errors.Is(err, context.Canceled) {
		return
	}

	c.logger.Error().Err(err).Msg("failed to process pending batch event")
	c.metrics.RecordError(ctx, c.Name())
}

func (c *batchCoordinator) processPendingEvent(ctx context.Context) error {
	event, batch, err := c.storage.TryGetPendingEvent(ctx)
	if err != nil {
		return fmt.Errorf("failed to load pending event from the storage: %w", err)
	}
	if event == nil {
		return nil
	}

	c.logger.Debug().
		Stringer(logging.FieldBatchId, event.BatchId).
		Stringer(logging.FieldBatchEventId, event.Id).
		Msg("processing batch event")

	handler, handlerIsDefined := c.handlers[event.NewStatus]

	if handlerIsDefined {
		err = c.handleEvent(ctx, handler, event, batch)
		if err != nil {
			return fmt.Errorf("failed to handle batch event, batchId=%s, eventId=%s %w", batch.Id, event.Id, err)
		}
		return nil
	}

	c.logger.Debug().
		Stringer(logging.FieldBatchId, event.BatchId).
		Stringer(logging.FieldBatchEventId, event.Id).
		Msgf("no batch event handler defined for status %s, skipping", event.NewStatus)

	err = c.storage.OnBatchEventSkipped(ctx, event.Id)
	if err != nil {
		return fmt.Errorf("failed to skip batch event, batchId=%s, eventId=%s: %w", batch.Id, event.Id, err)
	}

	return nil
}

func (c *batchCoordinator) handleEvent(
	ctx context.Context,
	handler BatchStatusChangeHandler,
	event *types.BatchEvent,
	batch *types.BlockBatch,
) error {
	updatedBatch, err := handler.HandleStateChange(ctx, batch)
	if err != nil {
		return fmt.Errorf("failed to handle state change: %w", err)
	}

	err = c.storage.OnBatchEventHandled(ctx, event.Id, updatedBatch)
	if err != nil {
		return fmt.Errorf("failed to update batch in the storage: %w", err)
	}

	c.logger.Debug().
		Stringer(logging.FieldBatchId, event.BatchId).
		Stringer(logging.FieldBatchEventId, event.Id).
		Msg("batch event is handled successfully")

	return nil
}
