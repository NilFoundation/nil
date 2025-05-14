package proofprovider

import (
	"context"
	"fmt"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/telemetry"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/executor"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/metrics"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/rpc"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/scheduler"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/srv"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/storage"
	"github.com/jonboulle/clockwork"
)

type Config struct {
	SyncCommitteeRpcEndpoint string            `yaml:"syncCommitteeEndpoint,omitempty"`
	OwnRpcEndpoint           string            `yaml:"ownEndpoint,omitempty"`
	SkipRate                 int               `yaml:"skipRate,omitempty"`
	MaxConcurrentBatches     uint32            `yaml:"maxConcurrentBatches,omitempty"`
	Telemetry                *telemetry.Config `yaml:",inline"`
}

func NewDefaultConfig() *Config {
	return &Config{
		SyncCommitteeRpcEndpoint: "tcp://127.0.0.1:8530",
		OwnRpcEndpoint:           "tcp://127.0.0.1:8531",
		SkipRate:                 0,
		MaxConcurrentBatches:     1,
		Telemetry: &telemetry.Config{
			ServiceName: "proof_provider",
		},
	}
}

type ProofProvider struct {
	srv.Service
}

func New(config *Config, database db.DB) (*ProofProvider, error) {
	logger := logging.NewLogger("proof_provider")

	if err := telemetry.Init(context.Background(), config.Telemetry); err != nil {
		logger.Error().Err(err).Msg("failed to initialize telemetry")
		return nil, err
	}
	metricsHandler, err := metrics.NewProofProviderMetrics()
	if err != nil {
		return nil, fmt.Errorf("error initializing metrics: %w", err)
	}

	clock := clockwork.NewRealClock()

	taskRpcClient := rpc.NewTaskRequestRpcClient(config.SyncCommitteeRpcEndpoint, logger)
	taskResultStorage := storage.NewTaskResultStorage(database, logger)
	taskResultSender := scheduler.NewTaskResultSender(taskRpcClient, taskResultStorage, metricsHandler, logger)

	taskStorage := storage.NewTaskStorage(database, clock, metricsHandler, logger)

	executorIdSource := executor.NewPersistentIdSource(
		storage.NewExecutorIdStorage(database, logger),
	)

	taskExecutor, err := executor.New(
		executor.DefaultConfig(),
		taskRpcClient,
		newTaskHandler(taskStorage, taskResultStorage, config.SkipRate, config.MaxConcurrentBatches, clock, logger),
		executorIdSource,
		metricsHandler,
		logger,
	)
	if err != nil {
		return nil, err
	}

	taskScheduler := scheduler.New(
		taskStorage,
		newTaskStateChangeHandler(taskResultStorage, executorIdSource, logger),
		metricsHandler,
		logger,
	)

	rpcServer := rpc.NewServerWithTasks(
		rpc.NewServerConfig(config.OwnRpcEndpoint),
		logger,
		taskScheduler,
		scheduler.NewTaskDebugger(taskStorage, logger),
	)

	taskCancelChecker := scheduler.NewTaskCancelChecker(
		taskRpcClient,
		taskStorage,
		executorIdSource,
		metricsHandler,
		logger,
	)

	return &ProofProvider{
		Service: srv.NewServiceWithHeartbeat(
			metricsHandler,
			logger,
			taskExecutor, taskScheduler, taskCancelChecker, taskResultSender, rpcServer,
		),
	}, nil
}
