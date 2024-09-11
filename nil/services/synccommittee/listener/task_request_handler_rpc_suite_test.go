package listener

import (
	"context"
	"crypto/rand"
	"math"
	"math/big"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/api"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	"github.com/NilFoundation/nil/nil/services/synccommittee/prover"
	"github.com/stretchr/testify/suite"
)

type TaskRequestHandlerTestSuite struct {
	suite.Suite
	context       context.Context
	cancellation  context.CancelFunc
	clientHandler api.TaskRequestHandler
	targetHandler *api.TaskRequestHandlerMock
}

func (s *TaskRequestHandlerTestSuite) SetupSuite() {
	s.context, s.cancellation = context.WithCancel(context.Background())
	listenerHttpEndpoint := "tcp://127.0.0.1:8530"
	s.targetHandler = newTaskRequestHandlerMock()

	go func() {
		err := runTaskListener(s.context, listenerHttpEndpoint, s.targetHandler)
		s.NoError(err)
	}()

	s.clientHandler = prover.NewTaskRequestRpcClient(
		listenerHttpEndpoint,
		logging.NewLogger("task-request-rpc-client"),
	)
}

func (s *TaskRequestHandlerTestSuite) TearDownSubTest() {
	s.targetHandler.ResetCalls()
}

func (s *TaskRequestHandlerTestSuite) TearDownSuite() {
	s.cancellation()
}

func runTaskListener(ctx context.Context, httpEndpoint string, handler api.TaskRequestHandler) error {
	taskListener := NewTaskListener(
		&TaskListenerConfig{HttpEndpoint: httpEndpoint},
		handler,
		logging.NewLogger("sync-committee-task-listener"),
	)

	return taskListener.Run(ctx)
}

func newTaskRequestHandlerMock() *api.TaskRequestHandlerMock {
	return &api.TaskRequestHandlerMock{
		GetTaskFunc: func(_ context.Context, request *api.TaskRequest) (*types.ProverTask, error) {
			predefinedTask := tasksForProvers[request.ProverId]
			return predefinedTask, nil
		},
		SetTaskResultFunc: func(ctx context.Context, result *types.ProverTaskResult) error {
			return nil
		},
	}
}

var (
	firstProverId  = types.ProverId(1)
	secondProverId = types.ProverId(2)
)

var tasksForProvers = map[types.ProverId]*types.ProverTask{
	firstProverId: {
		Id:            randomTaskId(),
		BatchNum:      1,
		BlockNum:      1,
		TaskType:      types.Preprocess,
		CircuitType:   types.Bytecode,
		Dependencies:  make(map[types.ProverTaskId]types.ProverTaskResult),
		DependencyNum: 0,
	},
	secondProverId: {
		Id:          randomTaskId(),
		BatchNum:    1234,
		BlockNum:    10,
		TaskType:    types.AggregatedFRI,
		CircuitType: types.ReadWrite,
		Dependencies: map[types.ProverTaskId]types.ProverTaskResult{
			types.ProverTaskId(10): types.SuccessTaskResult(
				types.ProverTaskId(10),
				randomProverId(),
				types.FinalProof,
				"2B3C4D5E",
			),
			types.ProverTaskId(20): types.SuccessTaskResult(
				types.ProverTaskId(20),
				randomProverId(),
				types.Commitment,
				"3C4D5E6F",
			),
		},
		DependencyNum: 2,
	},
}

func randomTaskId() types.ProverTaskId {
	bigInt, err := rand.Int(rand.Reader, big.NewInt(math.MaxInt32))
	if err != nil {
		panic(err)
	}
	return types.ProverTaskId(uint32(bigInt.Uint64()))
}

func randomProverId() types.ProverId {
	bigInt, err := rand.Int(rand.Reader, big.NewInt(math.MaxInt32))
	if err != nil {
		panic(err)
	}
	return types.ProverId(uint32(bigInt.Uint64()))
}
