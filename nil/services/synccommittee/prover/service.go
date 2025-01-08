package prover

import (
	"context"
	"fmt"

	"github.com/NilFoundation/nil/nil/client"
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/telemetry"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/executor"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/metrics"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/rpc"
	"github.com/rs/zerolog"
)

type Config struct {
	ProofProviderRpcEndpoint string
	NilRpcEndpoint           string
	Telemetry                *telemetry.Config
}

func NewDefaultConfig() *Config {
	return &Config{
		ProofProviderRpcEndpoint: "tcp://127.0.0.1:8531",
		NilRpcEndpoint:           "tcp://127.0.0.1:8529",
		Telemetry: &telemetry.Config{
			ServiceName: "prover",
		},
	}
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

	handler := newTaskHandler(
		taskRpcClient,
		common.NewTimer(),
		logger,
		defaultTaskHandlerConfig,
	)

	taskExecutor, err := executor.New(
		executor.DefaultConfig(),
		taskRpcClient,
		handler,
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

func NewRPCClient(endpoint string, logger zerolog.Logger) client.Client {
	return rpc.NewRetryClient(endpoint, logger)
}

func (p *Prover) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	defer telemetry.Shutdown(ctx)

	return p.taskExecutor.Run(ctx)
}
