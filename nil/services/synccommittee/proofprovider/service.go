package proofprovider

import (
	"context"
	"fmt"

	"github.com/NilFoundation/nil/nil/common/concurrent"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/executor"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/metrics"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/rpc"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/scheduler"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/storage"
	"github.com/rs/zerolog"
)

type Config struct {
	SyncCommitteeRpcEndpoint string
	OwnRpcEndpoint           string
	DbPath                   string
}

type ProofProvider struct {
	config        Config
	database      db.DB
	taskExecutor  executor.TaskExecutor
	taskScheduler scheduler.TaskScheduler
	taskListener  *rpc.TaskListener
	logger        zerolog.Logger
}

func New(config Config) (*ProofProvider, error) {
	database, err := db.NewBadgerDb(config.DbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	logger := logging.NewLogger("proof_provider")

	metricsHandler, err := metrics.NewHandler("sync_committee")
	if err != nil {
		return nil, fmt.Errorf("error initializing metrics: %s", err)
	}

	taskRpcClient := rpc.NewTaskRequestRpcClient(config.SyncCommitteeRpcEndpoint, logger)
	taskStorage := storage.NewTaskStorage(database, metricsHandler, logger)

	taskExecutor, err := executor.New(
		executor.DefaultConfig(),
		taskRpcClient,
		newTaskHandler(taskStorage, logger),
		logger,
	)
	if err != nil {
		return nil, err
	}

	taskScheduler := scheduler.New(
		taskStorage,
		newTaskStateChangeHandler(taskRpcClient, taskExecutor.Id(), logger),
		logger,
	)

	taskListener := rpc.NewTaskListener(
		&rpc.TaskListenerConfig{HttpEndpoint: config.OwnRpcEndpoint}, taskScheduler, logger,
	)

	return &ProofProvider{
		config:        config,
		database:      database,
		taskExecutor:  taskExecutor,
		taskScheduler: taskScheduler,
		taskListener:  taskListener,
		logger:        logger,
	}, nil
}

func (p *ProofProvider) Run(ctx context.Context) error {
	return concurrent.Run(
		ctx,
		p.taskExecutor.Run,
		p.taskListener.Run,
		p.taskScheduler.Run,
	)
}

func (p *ProofProvider) Close() {
	p.database.Close()
}
