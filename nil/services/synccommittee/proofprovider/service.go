package proofprovider

import (
	"context"
	"fmt"

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
)

type Config struct {
	srv.Config

	SyncCommitteeRpcEndpoint string
	TaskListenerRpcEndpoint  string
	Telemetry                *telemetry.Config
}

func NewDefaultConfig() *Config {
	return &Config{
		Config:                   srv.DefaultConfig(),
		SyncCommitteeRpcEndpoint: "tcp://127.0.0.1:8530",
		TaskListenerRpcEndpoint:  "tcp://127.0.0.1:8531",
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

	timer := common.NewTimer()

	taskRpcClient := rpc.NewTaskRequestRpcClient(config.SyncCommitteeRpcEndpoint, logger)
	taskResultStorage := storage.NewTaskResultStorage(database, logger)
	taskResultSender := scheduler.NewTaskResultSender(taskRpcClient, taskResultStorage, logger)

	taskStorage := storage.NewTaskStorage(database, timer, metricsHandler, logger)

	taskExecutor, err := executor.New(
		executor.DefaultConfig(),
		taskRpcClient,
		newTaskHandler(taskStorage, timer, logger),
		metricsHandler,
		logger,
	)
	if err != nil {
		return nil, err
	}

	taskScheduler := scheduler.New(
		taskStorage,
		newTaskStateChangeHandler(taskResultStorage, taskExecutor.Id(), logger),
		metricsHandler,
		logger,
	)

	taskListener := rpc.NewTaskListener(
		&rpc.TaskListenerConfig{HttpEndpoint: config.TaskListenerRpcEndpoint}, taskScheduler, logger,
	)

	return &ProofProvider{
		Service: srv.NewService(
			config.Config,
			logger,
			taskExecutor, taskScheduler, taskListener, taskResultSender,
		),
	}, nil
}
