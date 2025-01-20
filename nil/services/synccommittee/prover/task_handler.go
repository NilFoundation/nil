package prover

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/NilFoundation/nil/nil/client"
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/api"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/log"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/storage"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	"github.com/NilFoundation/nil/nil/services/synccommittee/prover/commands"
	"github.com/rs/zerolog"
)

type taskHandler struct {
	resultStorage storage.TaskResultStorage
	timer         common.Timer
	logger        zerolog.Logger
	config        taskHandlerConfig
	client        client.Client
}

func newTaskHandler(
	resultStorage storage.TaskResultStorage,
	timer common.Timer,
	logger zerolog.Logger,
	config taskHandlerConfig,
) api.TaskHandler {
	return &taskHandler{
		resultStorage: resultStorage,
		timer:         timer,
		logger:        logger,
		config:        config,
		client:        NewRPCClient(config.NilRpcEndpoint, logging.NewLogger("client")),
	}
}

type taskHandlerConfig = commands.CommandConfig

func newTaskHandlerConfig(nilRpcEndpoint string) taskHandlerConfig {
	return taskHandlerConfig{
		NilRpcEndpoint:      nilRpcEndpoint,
		ProofProducerBinary: "proof-producer-multi-threaded",
		OutDir:              os.TempDir(), // TODO: replace with shared folder
	}
}

func (h *taskHandler) Handle(ctx context.Context, executorId types.TaskExecutorId, task *types.Task) error {
	if task.TaskType == types.ProofBlock {
		err := types.UnexpectedTaskType(task)
		taskResult := types.NewFailureProverTaskResult(task.Id, executorId, fmt.Errorf("failed to create command for task: %w", err))
		log.NewTaskEvent(h.logger, zerolog.ErrorLevel, task).Err(err).Msg("failed to create command for task")
		return h.resultStorage.Put(ctx, taskResult)
	}
	commandFactory := commands.NewCommandFactory(h.config, h.logger)
	cmd, err := commandFactory.MakeHandlerCommandForTaskType(task.TaskType)
	if err != nil {
		taskResult := types.NewFailureProverTaskResult(task.Id, executorId, fmt.Errorf("unable to instantiate handler for task: %w", err))
		log.NewTaskEvent(h.logger, zerolog.ErrorLevel, task).
			Err(err).
			Msg("unable to instantiate handler for task")
		return h.resultStorage.Put(ctx, taskResult)
	}

	commandDefinition, err := cmd.MakeCommandDefinition(task)
	if err != nil {
		taskResult := types.NewFailureProverTaskResult(task.Id, executorId, fmt.Errorf("failed to create command for task: %w", err))
		log.NewTaskEvent(h.logger, zerolog.ErrorLevel, task).
			Err(err).
			Msg("failed to create command for task")
		return h.resultStorage.Put(ctx, taskResult)
	}
	startTime := h.timer.NowTime()
	log.NewTaskEvent(h.logger, zerolog.InfoLevel, task).Msg("Starting task execution")
	if before, ok := cmd.(commands.BeforeCommandExecuted); ok {
		log.NewTaskEvent(h.logger, zerolog.DebugLevel, task).Msg("Running action before task command")
		if err := before.BeforeCommandExecuted(ctx, task, commandDefinition.ExpectedResult); err != nil {
			log.NewTaskEvent(h.logger, zerolog.ErrorLevel, task).Msg("Action before task execution failed")
			return h.resultStorage.Put(ctx, types.NewFailureProverTaskResult(task.Id, executorId, fmt.Errorf("action before task execution failed: %w", err)))
		}
	}
	for _, execCmd := range commandDefinition.ExecCommands {
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		execCmd.Stdout = &stdout
		execCmd.Stderr = &stderr
		cmdString := strings.Join(execCmd.Args, " ")
		h.logger.Info().Msgf("Run command %v\n", cmdString)
		err := execCmd.Run()
		h.logger.Trace().Msgf("Task execution stdout:\n%v\n", stdout.String())
		if err != nil {
			taskResult := types.NewFailureProverTaskResult(task.Id, executorId, fmt.Errorf("task execution failed: %w", err))
			timeSpent := h.timer.NowTime().Sub(startTime)
			log.NewTaskEvent(h.logger, zerolog.ErrorLevel, task).
				Str("commandText", cmdString).
				Dur(logging.FieldTaskExecTime, timeSpent).
				Msgf("Task execution failed, stderr:\n%s\n", stderr.String())
			return h.resultStorage.Put(ctx, taskResult)
		}
	}

	taskBinaryResult := types.TaskResultData{}
	if after, ok := cmd.(commands.AfterCommandExecuted); ok {
		log.NewTaskEvent(h.logger, zerolog.DebugLevel, task).Msg("Running action after task command")
		taskBinaryResult, err = after.AfterCommandExecuted(task, commandDefinition.ExpectedResult)
		if err != nil {
			log.NewTaskEvent(h.logger, zerolog.ErrorLevel, task).Msg("Action after task command failed")
			return h.resultStorage.Put(ctx, types.NewFailureProverTaskResult(task.Id, executorId, fmt.Errorf("Action after task command failed: %w", err)))
		}
	}

	executionTime := h.timer.NowTime().Sub(startTime)
	log.NewTaskEvent(h.logger, zerolog.InfoLevel, task).
		Dur(logging.FieldTaskExecTime, executionTime).
		Msg("Task execution completed successfully")

	taskResult := types.NewSuccessProverTaskResult(task.Id, executorId, commandDefinition.ExpectedResult, taskBinaryResult)
	return h.resultStorage.Put(ctx, taskResult)
}
