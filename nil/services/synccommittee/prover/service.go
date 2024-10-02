package prover

import (
	"context"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/executor"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/rpc"
	"github.com/rs/zerolog"
)

type Config struct {
	ProofProviderRpcEndpoint string
}

type Prover struct {
	taskExecutor executor.TaskExecutor
	logger       zerolog.Logger
}

func New(config Config) (*Prover, error) {
	logger := logging.NewLogger("prover")

	taskRpcClient := rpc.NewTaskRequestRpcClient(config.ProofProviderRpcEndpoint, logger)

	taskExecutor, err := executor.New(
		executor.DefaultConfig(),
		taskRpcClient,
		newTaskHandler(taskRpcClient),
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
	return p.taskExecutor.Run(ctx)
}
