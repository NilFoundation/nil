package prover

import (
	"context"
	"fmt"

	"github.com/NilFoundation/nil/nil/client"
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/telemetry"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/executor"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/metrics"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/rpc"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/scheduler"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/srv"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/storage"
	"github.com/rs/zerolog"
)

type Config struct {
	srv.Config

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
	srv.Service
}

func New(config Config, database db.DB) (*Prover, error) {
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
	taskResultStorage := storage.NewTaskResultStorage(database, logger)
	taskResultSender := scheduler.NewTaskResultSender(taskRpcClient, taskResultStorage, logger)

	handler := newTaskHandler(
		taskResultStorage,
		common.NewTimer(),
		logger,
		newTaskHandlerConfig(config.NilRpcEndpoint),
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
		Service: srv.NewService(
			config.Config,
			logger,
			taskExecutor, taskResultSender,
		),
	}, nil
}

func NewRPCClient(endpoint string, logger zerolog.Logger) client.Client {
	return rpc.NewRetryClient(endpoint, logger)
}
