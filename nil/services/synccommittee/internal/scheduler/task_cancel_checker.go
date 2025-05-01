package scheduler

import (
	"context"
	"fmt"
	"time"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/api"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/executor"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/srv"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
)

type TaskCancelCheckerConfig struct {
	UpdateInterval time.Duration
}

func MakeDefaultCheckerConfig() TaskCancelCheckerConfig {
	return TaskCancelCheckerConfig{
		UpdateInterval: 60 * time.Second,
	}
}

type TaskSource interface {
	CancelTasksByParentId(ctx context.Context, isActive func(context.Context, types.TaskId) (bool, error)) (uint, error)
}

type TaskCancelChecker struct {
	srv.WorkerLoop

	requestHandler   api.TaskRequestHandler
	taskSource       TaskSource
	executorIdSource executor.IdSource
	config           TaskCancelCheckerConfig
}

func NewTaskCancelChecker(
	requestHandler api.TaskRequestHandler,
	taskSource TaskSource,
	executorIdSource executor.IdSource,
	metrics srv.WorkerMetrics,
	logger logging.Logger,
) *TaskCancelChecker {
	checker := &TaskCancelChecker{
		requestHandler:   requestHandler,
		taskSource:       taskSource,
		executorIdSource: executorIdSource,
		config:           MakeDefaultCheckerConfig(),
	}

	loopConfig := srv.NewWorkerLoopConfig(
		"task_cancel_checker",
		checker.config.UpdateInterval,
		checker.processRunningTasks,
	)
	checker.WorkerLoop = srv.NewWorkerLoop(loopConfig, metrics, logger)
	return checker
}

func (c *TaskCancelChecker) processRunningTasks(ctx context.Context) error {
	executorId, err := c.executorIdSource.GetCurrentId(ctx)
	if err != nil {
		return fmt.Errorf("failed to get current executor id: %w", err)
	}

	isTaskActiveFunc := func(ctx context.Context, id types.TaskId) (bool, error) {
		taskRequest := api.NewTaskCheckRequest(id, *executorId)
		exists, err := c.requestHandler.CheckIfTaskExists(ctx, taskRequest)
		if err != nil {
			return false, fmt.Errorf("failed to check task: %w", err)
		}
		return exists, nil
	}

	canceledCounter, err := c.taskSource.CancelTasksByParentId(ctx, isTaskActiveFunc)
	if err != nil {
		return fmt.Errorf("failed to cancel dead tasks: %w", err)
	}
	if canceledCounter == 0 {
		c.Logger.Debug().Msg("No task canceled")
	} else {
		c.Logger.Warn().Msgf("Canceled %d dead tasks", canceledCounter)
	}
	return nil
}
