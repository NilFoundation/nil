package prover

import (
	"context"
	"fmt"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/telemetry"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/executor"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/metrics"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/rpc"
	"github.com/rs/zerolog"
)

type Config struct {
	ProofProviderRpcEndpoint string
	Telemetry                *telemetry.Config
}

type Prover struct {
	taskExecutor executor.TaskExecutor
	logger       zerolog.Logger
}

func New(config Config) (*Prover, error) {
	logger := logging.NewLogger("prover")

	if err := telemetry.Init(context.Background(), config.Telemetry); err != nil {
		logger.Error().Err(err).Msg("failed to initialize telemetry")
		return nil, err
	}
	metricsHandler, err := metrics.NewProverMetrics()
	if err != nil {
		return nil, fmt.Errorf("error initializing metrics: %w", err)
	}

	taskRpcClient := rpc.NewTaskRequestRpcClient(config.ProofProviderRpcEndpoint, logger)

	defaultTaskHandlerConfig := taskHandlerConfig{
		AssignerBinary:      "echo",
		ProofProducerBinary: "echo",
		OutDir:              "/root/out",
	}

	taskExecutor, err := executor.New(
		executor.DefaultConfig(),
		taskRpcClient,
		newTaskHandler(taskRpcClient, logger, defaultTaskHandlerConfig),
		metricsHandler,
		logger,
	)
	if err != nil {
		return nil, err
	}

	return &Prover{
		taskExecutor: taskExecutor,
		logger:       logger,
	}, nil
}

func (p *Prover) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	defer telemetry.Shutdown(ctx)

	return p.taskExecutor.Run(ctx)
}
