package scheduler

import (
	"context"
	"fmt"
	"time"

	"github.com/NilFoundation/nil/nil/common/concurrent"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/api"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/log"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/srv"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/storage"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	"github.com/rs/zerolog"
)

type ResultSenderConfig struct {
	SendInterval time.Duration
}

func MakeDefaultSenderConfig() ResultSenderConfig {
	return ResultSenderConfig{
		SendInterval: 5 * time.Second,
	}
}

type TaskResultSender struct {
	requestHandler api.TaskRequestHandler
	storage        storage.TaskResultStorage
	logger         zerolog.Logger
	config         ResultSenderConfig
}

func NewTaskResultSender(
	requestHandler api.TaskRequestHandler,
	storage storage.TaskResultStorage,
	logger zerolog.Logger,
) *TaskResultSender {
	sender := &TaskResultSender{
		requestHandler: requestHandler,
		storage:        storage,
		config:         MakeDefaultSenderConfig(),
	}

	sender.logger = srv.LoggerWithWorkerName(logger, sender.Name())
	return sender
}

func (*TaskResultSender) Name() string {
	return "task_result_sender"
}

func (s *TaskResultSender) Run(ctx context.Context) error {
	s.logger.Debug().Msg("starting task result sender worker")

	concurrent.RunTickerLoop(ctx, s.config.SendInterval, func(ctx context.Context) {
		if err := s.processPendingResult(ctx); err != nil {
			s.logger.Error().Err(err).Msg("failed to send next task result")
		}
	})

	s.logger.Debug().Msg("task result sender worker stopped")
	return nil
}

func (s *TaskResultSender) processPendingResult(ctx context.Context) error {
	pendingResult, err := s.getPending(ctx)
	if err != nil || pendingResult == nil {
		return err
	}

	if err := s.sendPending(ctx, pendingResult); err != nil {
		return fmt.Errorf("taskId=%s: %w", pendingResult.TaskId, err)
	}

	return nil
}

func (s *TaskResultSender) getPending(ctx context.Context) (*types.TaskResult, error) {
	pendingResult, err := s.storage.TryGetPending(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get next task result from the storage: %w", err)
	}
	if pendingResult == nil {
		s.logger.Debug().Msg("no task result available, waiting for new one")
	}
	return pendingResult, nil
}

func (s *TaskResultSender) sendPending(ctx context.Context, result *types.TaskResult) error {
	log.NewTaskResultEvent(s.logger, zerolog.DebugLevel, result).Msg("sending task result")

	if err := s.requestHandler.SetTaskResult(ctx, result); err != nil {
		return fmt.Errorf("failed to send task result: %w", err)
	}

	log.NewTaskResultEvent(s.logger, zerolog.DebugLevel, result).Msg("task result successfully sent, deleting")

	if err := s.storage.Delete(ctx, result.TaskId); err != nil {
		return fmt.Errorf("failed to delete task result from storage: %w", err)
	}

	log.NewTaskResultEvent(s.logger, zerolog.DebugLevel, result).Msg("task result deleted from storage")
	return nil
}
