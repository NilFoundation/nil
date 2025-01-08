package proofprovider

import (
	"context"
	"testing"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/api"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/metrics"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/storage"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/testaide"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	"github.com/stretchr/testify/suite"
)

type TaskHandlerTestSuite struct {
	suite.Suite
	context      context.Context
	cancellation context.CancelFunc
	database     db.DB
	timer        common.Timer
	taskStorage  storage.TaskStorage
	taskHandler  api.TaskHandler
}

func (s *TaskHandlerTestSuite) SetupSuite() {
	s.context, s.cancellation = context.WithCancel(context.Background())

	var err error
	s.database, err = db.NewBadgerDbInMemory()
	s.Require().NoError(err)

	logger := logging.NewLogger("task_handler_test")

	metricsHandler, err := metrics.NewProofProviderMetrics()
	s.Require().NoError(err)

	s.taskStorage = storage.NewTaskStorage(s.database, common.NewTimer(), metricsHandler, logger)
	s.timer = testaide.NewTestTimer()
	s.taskHandler = newTaskHandler(s.taskStorage, s.timer, logger)
}

func TestTaskHandlerSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(TaskHandlerTestSuite))
}

func (s *TaskHandlerTestSuite) SetupTest() {
	err := s.database.DropAll()
	s.Require().NoError(err)
}

func (s *TaskHandlerTestSuite) TearDownSuite() {
	s.cancellation()
}

func (s *TaskHandlerTestSuite) TestReturnErrorOnUnexpectedTaskType() {
	testCases := []struct {
		name     string
		taskType types.TaskType
	}{
		{name: "PartialProve", taskType: types.PartialProve},
		{name: "FRIConsistencyChecks", taskType: types.FRIConsistencyChecks},
		{name: "MergeProof", taskType: types.MergeProof},
		{name: "AggregatedFRI", taskType: types.AggregatedFRI},
	}

	executorId := testaide.RandomExecutorId()

	for _, testCase := range testCases {
		s.Run(testCase.name, func() {
			task := testaide.NewTaskOfType(testCase.taskType)
			err := s.taskHandler.Handle(s.context, executorId, task)
			s.Require().ErrorIs(
				err,
				types.ErrUnexpectedTaskType,
				"taskHandler should have returned ErrUnexpectedTaskType on task of type %d", testCase.taskType,
			)
		})
	}
}

func (s *TaskHandlerTestSuite) TestHandleAggregateProofsTask() {
	now := s.timer.NowTime()
	executorId := testaide.RandomExecutorId()
	mainBlock := testaide.NewMainShardBlock()
	taskEntry := types.NewAggregateProofsTaskEntry(types.NewBatchId(), mainBlock, now)
	aggProofsTask := taskEntry.Task

	err := s.taskHandler.Handle(s.context, executorId, &taskEntry.Task)
	s.Require().NoError(err)

	otherExecutorId := testaide.RandomExecutorId()
	requestedTask, err := s.taskStorage.RequestTaskToExecute(s.context, otherExecutorId)
	s.Require().NoError(err)
	s.Require().NotNil(requestedTask)

	s.Require().NotEqual(aggProofsTask.Id, requestedTask.Id)
	s.Require().Equal(&aggProofsTask.Id, requestedTask.ParentTaskId)

	s.Require().Equal(aggProofsTask.BatchId, requestedTask.BatchId)
	s.Require().Equal(aggProofsTask.ShardId, requestedTask.ShardId)
	s.Require().Equal(aggProofsTask.BlockNum, requestedTask.BlockNum)
	s.Require().Equal(aggProofsTask.BlockHash, requestedTask.BlockHash)
}

func (s *TaskHandlerTestSuite) TestHandleBlockProofTask() {
	now := s.timer.NowTime()
	executorId := testaide.RandomExecutorId()
	execBlock := testaide.NewExecutionShardBlock()
	aggregateProofsEntry := types.NewAggregateProofsTaskEntry(types.NewBatchId(), execBlock, now)
	taskEntry, err := types.NewBlockProofTaskEntry(types.NewBatchId(), aggregateProofsEntry, execBlock, now)
	s.Require().NoError(err)

	err = s.taskHandler.Handle(s.context, executorId, &taskEntry.Task)
	s.Require().NoError(err, "taskHandler.Handle returned an error")

	// Extract 4 top-level tasks
	var ids [types.CircuitAmount]types.TaskId
	for i := range types.CircuitAmount {
		ids[i] = s.requestTask(executorId, true, types.PartialProve).Id
	}

	// Right now all remaining tasks should wait for dependencies
	s.requestTask(executorId, false, types.AggregatedChallenge)

	// Pass results for partial proof tasks
	for _, id := range ids {
		s.completeTask(executorId, id)
	}

	// Now only aggregate challenge task is available
	aggChallengeTask := s.requestTask(executorId, true, types.AggregatedChallenge)
	s.requestTask(executorId, false, types.CombinedQ)

	// After completion of aggregate challenge task we have combined Q tasks available
	s.completeTask(executorId, aggChallengeTask.Id)
	for i := range types.CircuitAmount {
		ids[i] = s.requestTask(executorId, true, types.CombinedQ).Id
	}
	s.requestTask(executorId, false, types.AggregatedFRI)

	// Pass results for combined Q tasks
	for _, id := range ids {
		s.completeTask(executorId, id)
	}

	// Now only aggregate FRI task is available
	aggFRITask := s.requestTask(executorId, true, types.AggregatedFRI)
	s.requestTask(executorId, false, types.FRIConsistencyChecks)

	// After completion of aggregate FRI task we have FRI consistency check tasks available
	s.completeTask(executorId, aggFRITask.Id)
	for i := range types.CircuitAmount {
		ids[i] = s.requestTask(executorId, true, types.FRIConsistencyChecks).Id
	}
	s.requestTask(executorId, false, types.MergeProof)

	// The only one waiting for dependencies is merge proof task
	for _, id := range ids {
		s.completeTask(executorId, id)
	}
	mpt := s.requestTask(executorId, true, types.MergeProof)
	s.completeTask(executorId, mpt.Id)

	// No more tasks for the block
	s.requestTask(executorId, false, types.PartialProve)
}

// Ensure that we have available task of certain type, or no tasks available
func (s *TaskHandlerTestSuite) requestTask(executorId types.TaskExecutorId, available bool, expectedType types.TaskType) *types.Task {
	s.T().Helper()
	t, err := s.taskStorage.RequestTaskToExecute(s.context, executorId)
	s.Require().NoError(err)
	if !available {
		s.Require().Nil(t)
		return nil
	}
	s.Require().NotNil(t)
	s.Equal(expectedType, t.TaskType)
	return t
}

// Set result for task
func (s *TaskHandlerTestSuite) completeTask(sender types.TaskExecutorId, id types.TaskId) {
	s.T().Helper()
	result := &types.TaskResult{TaskId: id, IsSuccess: true, Sender: sender}
	err := s.taskStorage.ProcessTaskResult(s.context, result)
	s.Require().NoError(err)
}
