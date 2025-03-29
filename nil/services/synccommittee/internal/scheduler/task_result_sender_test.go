package scheduler

import (
	"context"
	"errors"
	"testing"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/api"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/storage"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/testaide"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	"github.com/stretchr/testify/suite"
)

var errTestClientFailed = errors.New("task request handler failed")

type TaskResultSenderSuite struct {
	suite.Suite

	ctx    context.Context
	cancel context.CancelFunc

	database      db.DB
	resultStorage *storage.TaskResultStorage

	logger logging.Logger
}

func TestTaskResultSenderSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(TaskResultSenderSuite))
}

func (s *TaskResultSenderSuite) SetupSuite() {
	s.ctx, s.cancel = context.WithCancel(context.Background())

	database, err := db.NewBadgerDbInMemory()
	s.Require().NoError(err)
	s.database = database

	logger := logging.NewLogger("task_result_sender_test")
	s.resultStorage = storage.NewTaskResultStorage(database, logger)
}

func (s *TaskResultSenderSuite) TearDownSuite() {
	s.cancel()
}

func (s *TaskResultSenderSuite) TearDownTest() {
	err := s.database.DropAll()
	s.Require().NoError(err, "failed to clear database in TearDownTest")
}

func (s *TaskResultSenderSuite) Test_Send_Result_Empty_Storage() {
	handlerMock := &api.TaskRequestHandlerMock{}
	sender := s.newTestTaskResultSender(handlerMock)

	err := sender.processPendingResult(s.ctx)
	s.Require().NoError(err)
	s.Require().Zero(handlerMock.SetTaskResultCalls())
}

func (s *TaskResultSenderSuite) Test_Send_Result_Faulty_Client() {
	resultToSend := types.NewSuccessProviderTaskResult(types.NewTaskId(), testaide.RandomExecutorId(), nil, nil)
	err := s.resultStorage.Put(s.ctx, resultToSend)
	s.Require().NoError(err)

	handlerMock := &api.TaskRequestHandlerMock{
		SetTaskResultFunc: func(contextMoqParam context.Context, result *types.TaskResult) error {
			return errTestClientFailed
		},
	}

	sender := s.newTestTaskResultSender(handlerMock)

	err = sender.processPendingResult(s.ctx)
	s.Require().ErrorIs(err, errTestClientFailed)

	// Result was not removed from the storage
	storedResult, err := s.resultStorage.TryGetPending(s.ctx)
	s.Require().NoError(err)
	s.Require().Equal(resultToSend, storedResult)
}

func (s *TaskResultSenderSuite) Test_Send_And_Delete_Result() {
	resultToSend := types.NewSuccessProviderTaskResult(types.NewTaskId(), testaide.RandomExecutorId(), nil, nil)
	err := s.resultStorage.Put(s.ctx, resultToSend)
	s.Require().NoError(err)

	handlerMock := &api.TaskRequestHandlerMock{}

	sender := s.newTestTaskResultSender(handlerMock)

	err = sender.processPendingResult(s.ctx)
	s.Require().NoError(err)

	// Result was submitted via the client
	setResultCall := handlerMock.SetTaskResultCalls()
	s.Require().Len(setResultCall, 1)
	s.Require().Equal(resultToSend, setResultCall[0].Result)

	// Result was successfully removed from the storage
	storedResult, err := s.resultStorage.TryGetPending(s.ctx)
	s.Require().NoError(err)
	s.Require().Nil(storedResult)
}

func (s *TaskResultSenderSuite) newTestTaskResultSender(handler api.TaskRequestHandler) *TaskResultSender {
	s.T().Helper()
	return NewTaskResultSender(handler, s.resultStorage, s.logger)
}
