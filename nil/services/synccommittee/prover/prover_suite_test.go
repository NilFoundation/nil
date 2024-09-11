package prover

import (
	"context"
	"time"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/api"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	"github.com/stretchr/testify/suite"
)

type TestSuite struct {
	suite.Suite
	context       context.Context
	cancellation  context.CancelFunc
	prover        *Prover
	targetHandler *api.TaskRequestHandlerMock
}

func (s *TestSuite) SetupTest() {
	s.context, s.cancellation = context.WithCancel(context.Background())
	s.targetHandler = newTaskRequestHandlerMock()
	proverConfig := Config{
		TaskPollingInterval: 10 * time.Millisecond,
	}
	logger := logging.NewLogger("prover-test")
	newProver, err := NewProver(&proverConfig, s.targetHandler, NewTaskHandler(logger), logger)
	s.Require().NoError(err)
	s.prover = newProver
}

func (s *TestSuite) TearDownTest() {
	s.cancellation()
}

func newTaskRequestHandlerMock() *api.TaskRequestHandlerMock {
	taskId := types.ProverTaskId(0)
	return &api.TaskRequestHandlerMock{
		GetTaskFunc: func(_ context.Context, request *api.TaskRequest) (*types.ProverTask, error) {
			taskId++
			return &types.ProverTask{
				Id:            taskId,
				BatchNum:      1,
				BlockNum:      1,
				TaskType:      types.Preprocess,
				CircuitType:   types.Bytecode,
				Dependencies:  make(map[types.ProverTaskId]types.ProverTaskResult),
				DependencyNum: 0,
			}, nil
		},
		SetTaskResultFunc: func(ctx context.Context, result *types.ProverTaskResult) error {
			return nil
		},
	}
}
