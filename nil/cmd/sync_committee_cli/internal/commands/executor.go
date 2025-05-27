package commands

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/signal"
	"syscall"

	"github.com/NilFoundation/nil/nil/common/concurrent"
	"github.com/NilFoundation/nil/nil/common/logging"
)

type CmdParams interface {
	Validate() error
	GetExecutorParams() *ExecutorParams
}

type executor[P CmdParams] struct {
	writer io.StringWriter
	logger logging.Logger
	params P
}

type CmdOutput = string

const EmptyOutput = ""

func NewExecutor[P CmdParams](
	writer io.StringWriter,
	logger logging.Logger,
	params P,
) *executor[P] {
	return &executor[P]{
		writer: writer,
		logger: logger,
		params: params,
	}
}

func (t *executor[P]) Run(
	command func(context.Context) (CmdOutput, error),
) error {
	if err := t.params.Validate(); err != nil {
		return fmt.Errorf("invalid command params: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	executorParams := t.params.GetExecutorParams()

	runIteration := func(ctx context.Context) {
		output, err := command(ctx)
		if err != nil {
			t.onCommandError(err)
			return
		}

		_, err = t.writer.WriteString(output)
		if err != nil {
			t.logger.Error().Err(err).Msg("Failed to write command output")
		}
	}

	runIteration(ctx)

	if !executorParams.AutoRefresh {
		return nil
	}

	concurrent.RunTickerLoop(ctx, executorParams.RefreshInterval, func(ctx context.Context) {
		t.clearScreen()
		t.logger.Info().Msg("Refreshing data")
		runIteration(ctx)
	})

	return nil
}

// clearScreen clears terminal window using ANSI escape codes
func (t *executor[P]) clearScreen() {
	_, err := t.writer.WriteString("\033[H\033[2J")
	if err != nil {
		t.logger.Error().Err(err).Msg("failed to clear screen")
	}
}

func (t *executor[P]) onCommandError(err error) {
	switch {
	case err == nil:
		return

	case errors.Is(err, context.Canceled):
		t.logger.Info().Err(err).Msg("Command execution canceled")
		return

	case errors.Is(err, ErrNoDataFound):
		t.logger.Warn().Err(err).Msg("No data found")
		return

	default:
		t.logger.Err(err).Msg("Command execution failed")
	}
}
