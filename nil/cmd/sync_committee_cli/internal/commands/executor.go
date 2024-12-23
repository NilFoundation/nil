package commands

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/signal"
	"syscall"

	"github.com/NilFoundation/nil/nil/common/concurrent"
	"github.com/NilFoundation/nil/nil/services/synccommittee/debug"
	"github.com/NilFoundation/nil/nil/services/synccommittee/public"
	"github.com/rs/zerolog"
)

type CmdParams interface {
	Validate() error
	GetExecutorParams() *ExecutorParams
}

type Executor[P CmdParams] struct {
	writer io.StringWriter
	params P
	logger zerolog.Logger
}

type CmdOutput = string

const EmptyOutput = ""

func NewExecutor[P CmdParams](writer io.StringWriter, params P, logger zerolog.Logger) *Executor[P] {
	return &Executor[P]{
		writer: writer,
		params: params,
		logger: logger,
	}
}

func (t *Executor[P]) Run(
	command func(context.Context, P, public.TaskDebugApi) (CmdOutput, error),
) error {
	if err := t.params.Validate(); err != nil {
		return fmt.Errorf("invalid command params: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	executorParams := t.params.GetExecutorParams()
	client := debug.NewClient(executorParams.DebugRpcEndpoint, t.logger)

	runOnce := func(ctx context.Context) {
		output, err := command(ctx, t.params, client)
		if err != nil {
			t.onCommandError(err)
			return
		}

		_, err = t.writer.WriteString(output)
		if err != nil {
			t.logger.Error().Err(err).Msg("failed to write command output")
		}
	}

	runOnce(ctx)

	if !executorParams.AutoRefresh {
		return nil
	}

	concurrent.RunTickerLoop(ctx, executorParams.RefreshInterval, func(ctx context.Context) {
		t.clearScreen()
		t.logger.Info().Msg("refreshing data")
		runOnce(ctx)
	})

	return nil
}

// clearScreen clear terminal window using ANSI escape codes
func (t *Executor[P]) clearScreen() {
	_, err := t.writer.WriteString("\033[H\033[2J")
	if err != nil {
		t.logger.Error().Err(err).Msg("failed to clear screen")
	}
}

func (t *Executor[P]) onCommandError(err error) {
	if err == nil {
		return
	}

	var logLevel zerolog.Level
	switch {
	case errors.Is(err, context.Canceled):
		logLevel = zerolog.InfoLevel
	case errors.Is(err, ErrNoDataFound):
		logLevel = zerolog.WarnLevel
	default:
		logLevel = zerolog.ErrorLevel
	}

	t.logger.WithLevel(logLevel).Err(err).Msg("command execution failed")
}
