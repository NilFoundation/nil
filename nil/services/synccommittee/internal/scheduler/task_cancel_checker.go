package scheduler

import (
	"context"
	"fmt"
	"time"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/api"
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

	requestHandler api.TaskRequestHandler
	taskSource     TaskSource
	executorId     types.TaskExecutorId
	config         TaskCancelCheckerConfig
}

func NewTaskCancelChecker(
	requestHandler api.TaskRequestHandler,
	taskSource TaskSource,
	executorId types.TaskExecutorId,
	metrics srv.WorkerMetrics,
	logger logging.Logger,
) *TaskCancelChecker {
	checker := &TaskCancelChecker{
		requestHandler: requestHandler,
		taskSource:     taskSource,
		executorId:     executorId,
		config:         MakeDefaultCheckerConfig(),
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
	canceledCounter, err := c.taskSource.CancelTasksByParentId(ctx, c.isActive)
	if err != nil {
		return fmt.Errorf("failed to cancel dead tasks: %w", err)
	}
	if canceledCounter == 0 {
		c.Logger.Debug().Msg("no task canceled")
	} else {
		c.Logger.Warn().Msgf("canceled %d dead tasks", canceledCounter)
	}
	return nil
}

func (c *TaskCancelChecker) isActive(ctx context.Context, taskId types.TaskId) (bool, error) {
	taskRequest := api.NewTaskCheckRequest(taskId, c.executorId)
	isExists, err := c.requestHandler.CheckIfTaskExists(ctx, taskRequest)
	if err != nil {
		return isExists, fmt.Errorf("failed to check task: %w", err)
	}
	return isExists, nil
}
