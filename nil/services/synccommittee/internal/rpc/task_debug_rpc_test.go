package rpc

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/api"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/metrics"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/scheduler"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/storage"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/testaide"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	"github.com/stretchr/testify/suite"
)

type TaskSchedulerDebugRpcTestSuite struct {
	suite.Suite
	context      context.Context
	cancellation context.CancelFunc

	rpcClient api.TaskDebugApi
	scheduler scheduler.TaskScheduler

	database db.DB
	storage  storage.TaskStorage
}

func TestTaskSchedulerDebugRpcTestSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(TaskSchedulerDebugRpcTestSuite))
}

var (
	baseTime       = time.Date(2024, time.January, 1, 0, 0, 0, 0, time.UTC)
	someExecutor   = testaide.RandomExecutorId()
	running        = types.Running
	failed         = types.Failed
	proofBlockType = types.ProofBlock
	sortExecTime   = api.OrderByExecutionTime
	sortCreatedAt  = api.OrderByCreatedAt
	outputLimit    = 4
)

var entries = []*types.TaskEntry{
	testaide.GenerateTaskEntry(baseTime.Add(-6*time.Minute), running, testaide.RandomExecutorId()),
	testaide.GenerateTaskEntry(baseTime.Add(-4*time.Minute), running, someExecutor),
	testaide.GenerateTaskEntry(baseTime.Add(-8*time.Minute), running, someExecutor),
	testaide.GenerateTaskEntryOfType(proofBlockType, baseTime.Add(-2*time.Minute), failed, someExecutor),
	testaide.GenerateTaskEntryOfType(proofBlockType, baseTime.Add(-10*time.Minute), types.WaitingForInput, testaide.RandomExecutorId()),
	testaide.GenerateTaskEntry(baseTime, types.WaitingForExecutor, testaide.RandomExecutorId()),
}

func (s *TaskSchedulerDebugRpcTestSuite) SetupSuite() {
	s.context, s.cancellation = context.WithCancel(context.Background())
	const listenerEndpoint = "tcp://127.0.0.1:8532"

	logger := logging.NewLogger("task_debug_rpc_test")

	database, err := db.NewBadgerDbInMemory()
	s.Require().NoError(err)
	s.database = database
	metricsHandler, err := metrics.NewSyncCommitteeMetrics()
	s.Require().NoError(err)
	s.storage = storage.NewTaskStorage(s.database, metricsHandler, logger)

	s.scheduler = scheduler.New(
		s.storage,
		&api.TaskStateChangeHandlerMock{},
		metricsHandler,
		common.NewTestTimer(uint64(baseTime.Unix())),
		logger,
	)

	go func() {
		taskListener := NewTaskListener(
			&TaskListenerConfig{HttpEndpoint: listenerEndpoint},
			s.scheduler,
			logger,
		)

		err := taskListener.Run(s.context)
		s.NoError(err)
	}()

	err = testaide.WaitForEndpoint(s.context, listenerEndpoint)
	s.Require().NoError(err)

	s.rpcClient = NewTaskDebugRpcClient(listenerEndpoint, logger)
}

func (s *TaskSchedulerDebugRpcTestSuite) TearDownTest() {
	err := s.database.DropAll()
	s.Require().NoError(err, "failed to clear database in TearDownTest")
}

func (s *TaskSchedulerDebugRpcTestSuite) TearDownSuite() {
	s.cancellation()
}

func noFilterRequest() *api.TaskDebugRequest {
	return api.NewTaskDebugRequest(nil, nil, nil, nil, false, nil)
}

func (s *TaskSchedulerDebugRpcTestSuite) Test_Get_Tasks_Empty_Storage() {
	request := noFilterRequest()
	taskEntries, err := s.rpcClient.GetTasks(s.context, request)

	s.Require().NoError(err)
	s.Require().Empty(taskEntries)
}

func (s *TaskSchedulerDebugRpcTestSuite) Test_Get_Tasks() {
	err := s.storage.AddTaskEntries(s.context, entries)
	s.Require().NoError(err)

	testCases := []struct {
		name            string
		request         *api.TaskDebugRequest
		expectedResults []*types.TaskEntry
		ignoreOrder     bool
	}{
		{
			name:            "AllTasksNoFilter",
			request:         noFilterRequest(),
			expectedResults: entries,
			ignoreOrder:     true,
		},
		{
			name:            "FilterByExecutor",
			request:         api.NewTaskDebugRequest(nil, nil, &someExecutor, nil, false, nil),
			expectedResults: []*types.TaskEntry{entries[1], entries[2], entries[3]},
			ignoreOrder:     true,
		},
		{
			name:            "FilterByTaskType",
			request:         api.NewTaskDebugRequest(nil, &proofBlockType, nil, nil, false, nil),
			expectedResults: []*types.TaskEntry{entries[3], entries[4]},
			ignoreOrder:     true,
		},
		{
			name:            "FilterByStatusRunning",
			request:         api.NewTaskDebugRequest(&running, nil, nil, nil, false, nil),
			expectedResults: []*types.TaskEntry{entries[0], entries[1], entries[2]},
			ignoreOrder:     true,
		},
		{
			name:            "FilterByExecutorAndStatusFailed",
			request:         api.NewTaskDebugRequest(&failed, nil, &someExecutor, nil, false, nil),
			expectedResults: []*types.TaskEntry{entries[3]},
			ignoreOrder:     true,
		},
		{
			name:    "SortByCreatedAtAscending",
			request: api.NewTaskDebugRequest(nil, nil, nil, &sortCreatedAt, true, nil),
			expectedResults: []*types.TaskEntry{
				entries[4], entries[2], entries[0], entries[1], entries[3], entries[5],
			},
			ignoreOrder: false,
		},
		{
			name:    "SortByExecutionTimeDescendingAndLimit",
			request: api.NewTaskDebugRequest(nil, nil, nil, &sortExecTime, false, &outputLimit),
			expectedResults: []*types.TaskEntry{
				entries[3], entries[2], entries[0], entries[1],
			},
			ignoreOrder: false,
		},
		{
			name:    "SortByExecutionTimeDescendingAndLimit",
			request: api.NewTaskDebugRequest(nil, nil, nil, &sortExecTime, true, &outputLimit),
			expectedResults: []*types.TaskEntry{
				entries[1], entries[0], entries[2], entries[3],
			},
			ignoreOrder: false,
		},
	}

	for _, testCase := range testCases {
		s.Run(testCase.name, func() {
			result, err := s.rpcClient.GetTasks(s.context, testCase.request)
			s.Require().NoError(err)
			s.Require().NotNil(result)

			if testCase.ignoreOrder {
				s.Require().ElementsMatch(testCase.expectedResults, result)
			} else {
				s.Require().Equal(testCase.expectedResults, result)
			}
		})
	}
}

func (s *TaskSchedulerDebugRpcTestSuite) Test_Get_Task_Tree_Empty_Storage() {
	taskTree, err := s.rpcClient.GetTaskTree(s.context, types.NewTaskId())
	s.Require().NoError(err)
	s.Require().Nil(taskTree)
}

func (s *TaskSchedulerDebugRpcTestSuite) Test_Get_Task_Tree_Not_Found() {
	err := s.storage.AddTaskEntries(s.context, entries)
	s.Require().NoError(err)

	someRootId := types.NewTaskId()
	taskTree, err := s.rpcClient.GetTaskTree(s.context, someRootId)
	s.Require().NoError(err)
	s.Require().Nil(taskTree)
}

func (s *TaskSchedulerDebugRpcTestSuite) Test_Get_Task_Tree_No_Dependencies() {
	entry := testaide.GenerateTaskEntry(baseTime, running, testaide.RandomExecutorId())
	err := s.storage.AddSingleTaskEntry(s.context, *entry)
	s.Require().NoError(err)

	taskTree, err := s.rpcClient.GetTaskTree(s.context, entry.Task.Id)
	s.Require().NoError(err)
	s.Require().NotNil(taskTree)

	s.Require().Equal(entry, &taskTree.TaskEntry)
	s.Require().Nil(taskTree.Result)
	s.Require().Empty(taskTree.Dependencies)
}

func (s *TaskSchedulerDebugRpcTestSuite) Test_Get_Task_Tree_With_Dependencies() {
	//   A
	//  / \
	// B   C
	//  \ / \
	//   D   E

	taskA := testaide.GenerateTaskEntry(baseTime, types.WaitingForInput, testaide.RandomExecutorId())

	taskB := testaide.GenerateTaskEntry(baseTime, types.WaitingForInput, testaide.RandomExecutorId())
	taskA.AddDependency(taskB)

	taskC := testaide.GenerateTaskEntry(baseTime, types.WaitingForInput, testaide.RandomExecutorId())
	taskA.AddDependency(taskC)

	taskD := testaide.GenerateTaskEntry(baseTime, types.Running, testaide.RandomExecutorId())
	taskB.AddDependency(taskD)
	taskC.AddDependency(taskD)

	taskE := testaide.GenerateTaskEntry(baseTime, types.Running, testaide.RandomExecutorId())
	taskC.AddDependency(taskE)

	err := s.storage.AddTaskEntries(s.context, []*types.TaskEntry{taskA, taskB, taskC, taskD, taskE})
	s.Require().NoError(err)

	taskTree, err := s.rpcClient.GetTaskTree(s.context, taskA.Task.Id)
	s.Require().NoError(err)
	s.Require().NotNil(taskTree)

	s.Require().Equal(taskA, &taskTree.TaskEntry)
	s.Require().Nil(taskTree.Result)
	s.Require().Len(taskTree.Dependencies, 2)

	taskBSubtree := s.getDependencyById(taskTree, taskB.Task.Id)
	s.Require().Equal(taskB, &taskBSubtree.TaskEntry)

	s.Require().Len(taskBSubtree.Dependencies, 1)
	s.Require().Equal(taskD, &taskBSubtree.Dependencies[0].TaskEntry)
	s.Require().Empty(taskBSubtree.Dependencies[0].Dependencies)

	taskCSubtree := s.getDependencyById(taskTree, taskC.Task.Id)
	s.Require().Equal(taskC, &taskCSubtree.TaskEntry)

	s.Require().ElementsMatch(
		[]*types.TaskEntry{taskD, taskE},
		[]*types.TaskEntry{&taskCSubtree.Dependencies[0].TaskEntry, &taskCSubtree.Dependencies[1].TaskEntry},
	)

	s.Require().Empty(taskCSubtree.Dependencies[0].Dependencies)
	s.Require().Empty(taskCSubtree.Dependencies[1].Dependencies)
}

func (s *TaskSchedulerDebugRpcTestSuite) getDependencyById(taskTree *types.TaskTree, taskId types.TaskId) *types.TaskTree {
	s.T().Helper()

	actualTaskIdx := slices.IndexFunc(taskTree.Dependencies, func(child *types.TaskTree) bool {
		return child.TaskEntry.Task.Id == taskId
	})
	s.Require().NotEqual(-1, actualTaskIdx)

	return taskTree.Dependencies[actualTaskIdx]
}
