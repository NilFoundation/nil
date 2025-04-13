package prover

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/NilFoundation/nil/nil/client/rpc"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/api"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/log"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	"github.com/NilFoundation/nil/nil/services/synccommittee/prover/commands"
	"github.com/NilFoundation/nil/nil/services/synccommittee/prover/internal/constants"
	"github.com/jonboulle/clockwork"
	"github.com/rs/zerolog"
)

type TaskResultSaver interface {
	Put(ctx context.Context, result *types.TaskResult) error
}

type taskHandler struct {
	resultSaver TaskResultSaver
	clock       clockwork.Clock
	logger      logging.Logger
	config      taskHandlerConfig
}

func newTaskHandler(
	resultSaver TaskResultSaver,
	clock clockwork.Clock,
	logger logging.Logger,
	config taskHandlerConfig,
) api.TaskHandler {
	return &taskHandler{
		resultSaver: resultSaver,
		clock:       clock,
		logger:      logger,
		config:      config,
	}
}

type taskHandlerConfig = commands.CommandConfig

type executionResult struct {
	artifacts  types.TaskOutputArtifacts
	binaryData types.TaskResultData
}

func newTaskHandlerConfig(nilRpcEndpoint string) taskHandlerConfig {
	return taskHandlerConfig{
		NilRpcEndpoint:      nilRpcEndpoint,
		ProofProducerBinary: "proof-producer-multi-threaded",
		OutDir:              os.TempDir(), // TODO: replace with shared folder
	}
}

func (h *taskHandler) IsReadyToHandle(ctx context.Context) (bool, error) {
	// Prover is always ready to pick up a new task after finishing a previous one
	return true, nil
}

func (h *taskHandler) Handle(ctx context.Context, executorId types.TaskExecutorId, task *types.Task) error {
	var taskResult *types.TaskResult

	execResult, err := h.handleImpl(ctx, task)
	if err == nil {
		log.NewTaskEvent(h.logger, zerolog.InfoLevel, task).Msg("task execution completed successfully")
		taskResult = types.NewSuccessProverTaskResult(task.Id, executorId, execResult.artifacts, execResult.binaryData)
	} else {
		log.NewTaskEvent(h.logger, zerolog.ErrorLevel, task).Err(err).Msg("task execution failed")
		taskResult = types.NewFailureProverTaskResult(task.Id, executorId, h.mapErrToTaskExec(err))
	}

	return h.resultSaver.Put(ctx, taskResult)
}

func (h *taskHandler) handleImpl(ctx context.Context, task *types.Task) (*executionResult, error) {
	if task.TaskType == types.ProofBatch {
		return nil, types.NewTaskErrNotSupportedType(task.TaskType)
	}

	commandFactory := commands.NewCommandFactory(h.config, h.logger)
	cmd, err := commandFactory.MakeHandlerCommandForTaskType(task.TaskType)
	if err != nil {
		return nil, fmt.Errorf("unable to instantiate handler for task: %w", err)
	}

	commandDefinition, err := cmd.MakeCommandDefinition(task)
	if err != nil {
		return nil, fmt.Errorf("failed to create command for task: %w", err)
	}

	log.NewTaskEvent(h.logger, zerolog.InfoLevel, task).Msg("Starting task execution")

	if before, ok := cmd.(commands.BeforeCommandExecuted); ok {
		log.NewTaskEvent(h.logger, zerolog.DebugLevel, task).Msg("Running action before task command")
		if err := before.BeforeCommandExecuted(ctx, task, commandDefinition.ExpectedResult); err != nil {
			return nil, fmt.Errorf("action before task command failed: %w", err)
		}
	}

	for _, execCmd := range commandDefinition.ExecCommands {
		if err := h.executeCommand(execCmd); err != nil {
			return nil, fmt.Errorf("command execution failed: %w", err)
		}
	}

	taskBinaryResult := types.TaskResultData{}
	if after, ok := cmd.(commands.AfterCommandExecuted); ok {
		log.NewTaskEvent(h.logger, zerolog.DebugLevel, task).Msg("Running action after task command")
		taskBinaryResult, err = after.AfterCommandExecuted(task, commandDefinition.ExpectedResult)
		if err != nil {
			return nil, fmt.Errorf("action after task command failed: %w", err)
		}
	}

	return &executionResult{
		artifacts:  commandDefinition.ExpectedResult,
		binaryData: taskBinaryResult,
	}, nil
}

func (h *taskHandler) executeCommand(execCmd *exec.Cmd) error {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	execCmd.Stdout = &stdout
	execCmd.Stderr = &stderr
	cmdString := strings.Join(execCmd.Args, " ")

	h.logger.Info().Msgf("Run command %v\n", cmdString)

	startTime := h.clock.Now()
	err := execCmd.Run()
	h.logger.Trace().Msgf("Task execution stdout:\n%v\n", stdout.String())
	execTime := h.clock.Now().Sub(startTime)

	if err == nil {
		h.logger.Info().
			Dur("commandExecTime", execTime).
			Msg("Command execution completed successfully")
		return nil
	}

	h.logger.Error().
		Err(err).
		Str("commandText", cmdString).
		Dur("commandExecTime", execTime).
		Msgf("Command execution failed, stderr:\n%s\n", stderr.String())
	return err
}

func (h *taskHandler) mapErrToTaskExec(err error) *types.TaskExecError {
	var taskExecError *types.TaskExecError
	var exitErr *exec.ExitError

	switch {
	case err == nil:
		return nil

	case errors.As(err, &taskExecError):
		return taskExecError

	case errors.As(err, &exitErr):
		return h.mapCmdExitErrToTaskExec(exitErr)

	case errors.As(err, new(rpc.CallError)):
		return types.NewTaskExecErrorf(types.TaskErrRpc, "%s", err)

	default:
		return types.NewTaskErrUnknown(err)
	}
}

func (h *taskHandler) mapCmdExitErrToTaskExec(exitErr *exec.ExitError) *types.TaskExecError {
	status, ok := exitErr.Sys().(syscall.WaitStatus)
	if !ok {
		h.logger.Warn().Err(exitErr).Msg("failed to get syscall.WaitStatus from exec.ExitError")
		return types.NewTaskErrUnknown(exitErr)
	}

	if status.Signaled() {
		signal := status.Signal()
		return types.NewTaskExecErrorf(types.TaskErrTerminated, "process terminated by signal %s", signal)
	}

	resultCode := constants.ProofProducerResultCode(status.ExitStatus())
	var taskErrType types.TaskErrType
	if errType, ok := constants.ProofProducerErrors[resultCode]; ok {
		taskErrType = errType
	} else {
		taskErrType = types.TaskErrUnknown
	}

	return types.NewTaskExecErrorf(taskErrType, "process exited with code %d", resultCode)
}
