package rpc

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/services/rpc"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/api"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/metrics"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/scheduler"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/storage"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/testaide"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	"github.com/NilFoundation/nil/nil/services/synccommittee/public"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/suite"
)

type TaskSchedulerDebugRpcTestSuite struct {
	suite.Suite
	context      context.Context
	cancellation context.CancelFunc

	rpcClient public.TaskDebugApi
	scheduler scheduler.TaskScheduler

	database db.DB
	storage  *storage.TaskStorage
	clock    *clockwork.FakeClock
}

func TestTaskSchedulerDebugRpcTestSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(TaskSchedulerDebugRpcTestSuite))
}

var (
	someExecutor   = testaide.RandomExecutorId()
	running        = types.Running
	failed         = types.Failed
	proofBatchType = types.ProofBatch
	sortExecTime   = public.OrderByExecutionTime
	sortCreatedAt  = public.OrderByCreatedAt
	outputLimit    = 4
)

func newTaskEntries(now time.Time) []*types.TaskEntry {
	return []*types.TaskEntry{
		testaide.NewTaskEntry(now.Add(-6*time.Minute), running, testaide.RandomExecutorId()),
		testaide.NewTaskEntry(now.Add(-4*time.Minute), running, someExecutor),
		testaide.NewTaskEntry(now.Add(-8*time.Minute), running, someExecutor),
		testaide.NewTaskEntryOfType(proofBatchType, now.Add(-2*time.Minute), failed, someExecutor),
		testaide.NewTaskEntryOfType(
			proofBatchType, now.Add(-10*time.Minute), types.WaitingForInput, testaide.RandomExecutorId()),
		testaide.NewTaskEntry(now, types.WaitingForExecutor, testaide.RandomExecutorId()),
	}
}

func (s *TaskSchedulerDebugRpcTestSuite) SetupSuite() {
	s.context, s.cancellation = context.WithCancel(context.Background())
	listenerEndpoint := rpc.GetSockPath(s.T())

	logger := logging.NewLogger("task_debug_rpc_test")

	database, err := db.NewBadgerDbInMemory()
	s.Require().NoError(err)
	s.database = database
	metricsHandler, err := metrics.NewSyncCommitteeMetrics()
	s.Require().NoError(err)

	s.clock = testaide.NewTestClock()

	s.storage = storage.NewTaskStorage(
		s.database,
		s.clock,
		metricsHandler,
		logger,
	)

	s.scheduler = scheduler.New(
		s.storage,
		&api.TaskStateChangeHandlerMock{},
		metricsHandler,
		logger,
	)

	started := make(chan struct{})
	go func() {
		taskListener := NewTaskListener(
			&TaskListenerConfig{HttpEndpoint: listenerEndpoint},
			s.scheduler,
			logger,
		)

		err := taskListener.Run(s.context, started)
		s.NoError(err)
	}()
	err = testaide.WaitFor(s.context, started, 10*time.Second)
	s.Require().NoError(err, "task listener did not start in time")

	s.rpcClient = NewTaskDebugRpcClient(listenerEndpoint, logger)
}

func (s *TaskSchedulerDebugRpcTestSuite) TearDownTest() {
	testaide.ResetTestClock(s.clock)
	err := s.database.DropAll()
	s.Require().NoError(err, "failed to clear database in TearDownTest")
}

func (s *TaskSchedulerDebugRpcTestSuite) TearDownSuite() {
	s.cancellation()
}

func noFilterRequest() *public.TaskDebugRequest {
	return public.NewTaskDebugRequest(nil, nil, nil, nil, false, nil)
}

func (s *TaskSchedulerDebugRpcTestSuite) Test_Get_Tasks_Empty_Storage() {
	request := noFilterRequest()
	taskEntries, err := s.rpcClient.GetTasks(s.context, request)

	s.Require().NoError(err)
	s.Require().Empty(taskEntries)
}

type getTaskTestCase struct {
	name            string
	request         *public.TaskDebugRequest
	expectedResults []*types.TaskEntry
	ignoreOrder     bool
}

func (s *TaskSchedulerDebugRpcTestSuite) Test_Get_Tasks() {
	entries := newTaskEntries(s.clock.Now())
	err := s.storage.AddTaskEntries(s.context, entries...)
	s.Require().NoError(err)

	testCases := []getTaskTestCase{
		{
			name:            "AllTasksNoFilter",
			request:         noFilterRequest(),
			expectedResults: entries,
			ignoreOrder:     true,
		},
		{
			name:            "FilterByExecutor",
			request:         public.NewTaskDebugRequest(nil, nil, &someExecutor, nil, false, nil),
			expectedResults: []*types.TaskEntry{entries[1], entries[2], entries[3]},
			ignoreOrder:     true,
		},
		{
			name:            "FilterByTaskType",
			request:         public.NewTaskDebugRequest(nil, &proofBatchType, nil, nil, false, nil),
			expectedResults: []*types.TaskEntry{entries[3], entries[4]},
			ignoreOrder:     true,
		},
		{
			name:            "FilterByStatusRunning",
			request:         public.NewTaskDebugRequest(&running, nil, nil, nil, false, nil),
			expectedResults: []*types.TaskEntry{entries[0], entries[1], entries[2]},
			ignoreOrder:     true,
		},
		{
			name:            "FilterByExecutorAndStatusFailed",
			request:         public.NewTaskDebugRequest(&failed, nil, &someExecutor, nil, false, nil),
			expectedResults: []*types.TaskEntry{entries[3]},
			ignoreOrder:     true,
		},
		{
			name:    "SortByCreatedAtAscending",
			request: public.NewTaskDebugRequest(nil, nil, nil, &sortCreatedAt, true, nil),
			expectedResults: []*types.TaskEntry{
				entries[4], entries[2], entries[0], entries[1], entries[3], entries[5],
			},
			ignoreOrder: false,
		},
		{
			name:    "SortByExecutionTimeDescendingAndLimit",
			request: public.NewTaskDebugRequest(nil, nil, nil, &sortExecTime, false, &outputLimit),
			expectedResults: []*types.TaskEntry{
				entries[3], entries[2], entries[0], entries[1],
			},
			ignoreOrder: false,
		},
		{
			name:    "SortByExecutionTimeDescendingAndLimit",
			request: public.NewTaskDebugRequest(nil, nil, nil, &sortExecTime, true, &outputLimit),
			expectedResults: []*types.TaskEntry{
				entries[1], entries[0], entries[2], entries[3],
			},
			ignoreOrder: false,
		},
	}

	for _, testCase := range testCases {
		s.Run(testCase.name, func() {
			s.runGetTasksCase(testCase)
		})
	}
}

func (s *TaskSchedulerDebugRpcTestSuite) runGetTasksCase(testCase getTaskTestCase) {
	s.T().Helper()

	result, err := s.rpcClient.GetTasks(s.context, testCase.request)
	s.Require().NoError(err)
	s.Require().NotNil(result)

	s.Require().Len(result, len(testCase.expectedResults))

	if testCase.ignoreOrder {
		slices.SortFunc(testCase.expectedResults, func(a, b *types.TaskEntry) int {
			return strings.Compare(a.Task.Id.String(), b.Task.Id.String())
		})
		slices.SortFunc(result, func(a, b *public.TaskView) int {
			return strings.Compare(a.Id.String(), b.Id.String())
		})
	}

	for i, taskView := range result {
		taskEntry := testCase.expectedResults[i]
		s.requireTaskViewEqual(taskEntry, taskView)
	}
}

func (s *TaskSchedulerDebugRpcTestSuite) Test_Get_Task_Tree_Empty_Storage() {
	taskTree, err := s.rpcClient.GetTaskTree(s.context, types.NewTaskId())
	s.Require().NoError(err)
	s.Require().Nil(taskTree)
}

func (s *TaskSchedulerDebugRpcTestSuite) Test_Get_Task_Tree_Not_Found() {
	entries := newTaskEntries(s.clock.Now())
	err := s.storage.AddTaskEntries(s.context, entries...)
	s.Require().NoError(err)

	someRootId := types.NewTaskId()
	taskTree, err := s.rpcClient.GetTaskTree(s.context, someRootId)
	s.Require().NoError(err)
	s.Require().Nil(taskTree)
}

func (s *TaskSchedulerDebugRpcTestSuite) Test_Get_Task_Tree_No_Dependencies() {
	now := s.clock.Now()
	entry := testaide.NewTaskEntry(now, running, testaide.RandomExecutorId())
	err := s.storage.AddTaskEntries(s.context, entry)
	s.Require().NoError(err)

	taskTree, err := s.rpcClient.GetTaskTree(s.context, entry.Task.Id)
	s.Require().NoError(err)
	s.Require().NotNil(taskTree)

	s.requireTaskTreeViewEqual(entry, taskTree)
	s.Require().Empty(taskTree.ResultErrorText)
	s.Require().Empty(taskTree.Dependencies)
}

func (s *TaskSchedulerDebugRpcTestSuite) Test_Get_Task_Tree_With_Dependencies() {
	//   A
	//  / \
	// B   C
	//  \ / \
	//   D   E

	now := s.clock.Now()

	taskA := testaide.NewTaskEntry(now, types.WaitingForInput, testaide.RandomExecutorId())

	taskB := testaide.NewTaskEntry(now, types.WaitingForInput, testaide.RandomExecutorId())
	taskA.AddDependency(taskB)

	taskC := testaide.NewTaskEntry(now, types.WaitingForInput, testaide.RandomExecutorId())
	taskA.AddDependency(taskC)

	taskD := testaide.NewTaskEntry(now, types.Running, testaide.RandomExecutorId())
	taskB.AddDependency(taskD)
	taskC.AddDependency(taskD)

	taskE := testaide.NewTaskEntry(now, types.Running, testaide.RandomExecutorId())
	taskC.AddDependency(taskE)

	err := s.storage.AddTaskEntries(s.context, taskA, taskB, taskC, taskD, taskE)
	s.Require().NoError(err)

	taskATree, err := s.rpcClient.GetTaskTree(s.context, taskA.Task.Id)
	s.Require().NoError(err)
	s.Require().NotNil(taskATree)

	s.requireTaskTreeViewEqual(taskA, taskATree)
	s.Require().Empty(taskATree.ResultErrorText)
	s.Require().Len(taskATree.Dependencies, 2, "task A should have 2 dependencies")
	s.requireHasDependency(taskATree, taskB)
	s.requireHasDependency(taskATree, taskC)

	taskBSubtree := taskATree.Dependencies[taskB.Task.Id]
	s.Require().Len(taskBSubtree.Dependencies, 1, "task B should have 1 dependency")
	s.requireHasDependency(taskBSubtree, taskD)

	taskCSubtree := taskATree.Dependencies[taskC.Task.Id]
	s.Require().Len(taskCSubtree.Dependencies, 2, "task C should have 2 dependencies")
	s.requireHasDependency(taskCSubtree, taskD)
	s.requireHasDependency(taskCSubtree, taskE)

	taskDSubtree := taskBSubtree.Dependencies[taskD.Task.Id]
	s.Require().Empty(taskDSubtree.Dependencies, "task D should have no dependencies")

	taskESubtree := taskCSubtree.Dependencies[taskE.Task.Id]
	s.Require().Empty(taskESubtree.Dependencies, "task E should have no dependencies")
}

func (s *TaskSchedulerDebugRpcTestSuite) Test_Get_Task_Tree_With_Terminated_Dependencies() {
	//   A
	//  / \
	// B   C

	now := s.clock.Now()

	taskA := testaide.NewTaskEntry(now.Add(-10*time.Minute), types.WaitingForInput, types.UnknownExecutorId)

	taskB := testaide.NewTaskEntry(now.Add(-10*time.Minute), types.WaitingForExecutor, types.UnknownExecutorId)
	taskA.AddDependency(taskB)

	taskC := testaide.NewTaskEntry(now.Add(-1*time.Minute), types.WaitingForExecutor, types.UnknownExecutorId)
	taskA.AddDependency(taskC)

	err := s.storage.AddTaskEntries(s.context, taskA, taskB, taskC)
	s.Require().NoError(err)

	executor := testaide.RandomExecutorId()

	s.requestAndSendResult(&taskB.Task, executor, true)
	s.requestAndSendResult(&taskC.Task, executor, false)

	taskATree, err := s.rpcClient.GetTaskTree(s.context, taskA.Task.Id)
	s.Require().NoError(err)
	s.Require().NotNil(taskATree)

	s.requireTaskTreeViewEqual(taskA, taskATree)
	s.Require().Empty(taskATree.ResultErrorText)
	s.Require().Len(taskATree.Dependencies, 2)

	s.requireHasTerminatedLeafDependency(taskATree, executor, taskB, true)
	s.requireHasTerminatedLeafDependency(taskATree, executor, taskC, false)
}

func (s *TaskSchedulerDebugRpcTestSuite) requestAndSendResult(
	expected *types.Task, executor types.TaskExecutorId, completeSuccessfully bool,
) {
	s.T().Helper()

	taskToExec, err := s.storage.RequestTaskToExecute(s.context, executor)
	s.Require().NoError(err)
	s.Require().NotNil(taskToExec)
	s.Require().Equal(expected, taskToExec)

	// emulate time progress for non-zero Task.ExecutionTime results
	s.clock.Advance(10 * time.Minute)

	var taskResult *types.TaskResult
	if completeSuccessfully {
		taskResult = testaide.NewSuccessTaskResult(taskToExec.Id, executor)
	} else {
		taskResult = testaide.NewNonRetryableErrorTaskResult(taskToExec.Id, executor)
	}

	err = s.storage.ProcessTaskResult(s.context, taskResult)
	s.Require().NoError(err)
}

func (s *TaskSchedulerDebugRpcTestSuite) requireHasTerminatedLeafDependency(
	tree *public.TaskTreeView, executor public.TaskExecutorId, expected *types.TaskEntry, successResult bool,
) {
	s.T().Helper()

	taskSubtree, exists := tree.Dependencies[expected.Task.Id]
	s.Require().True(exists)
	s.Require().Empty(taskSubtree.Dependencies)

	var expectedStatus public.TaskStatus
	if successResult {
		expectedStatus = types.Completed
	} else {
		expectedStatus = types.Failed
	}

	assertions := []bool{
		s.Equal(expected.Task.Id, taskSubtree.Id),
		s.Equal(expected.Task.TaskType, taskSubtree.Type),
		s.Equal(expected.Task.CircuitType, taskSubtree.CircuitType),
		s.NotEmpty(taskSubtree.ExecutionTime),
		s.Equal(expectedStatus, taskSubtree.Status),
		s.Equal(executor, taskSubtree.Owner),
		s.Equal(successResult, taskSubtree.ResultErrorText == ""),
	}

	s.requireAllPass(assertions, expected.Task.Id)
}

func (s *TaskSchedulerDebugRpcTestSuite) requireTaskViewEqual(expected *types.TaskEntry, actual *public.TaskView) {
	s.T().Helper()
	s.Require().NotNil(actual)

	assertions := s.commonTaskAssertions(expected, &actual.TaskViewCommon)

	assertions = append(assertions, []bool{
		s.Equal(expected.Task.BatchId, actual.BatchId),

		s.Equal(expected.Created, actual.CreatedAt),
		s.Equal(expected.Started, actual.StartedAt),
	}...)

	s.requireAllPass(assertions, expected.Task.Id)
}

func (s *TaskSchedulerDebugRpcTestSuite) requireHasDependency(tree *public.TaskTreeView, expected *types.TaskEntry) {
	s.T().Helper()
	taskSubtree, exists := tree.Dependencies[expected.Task.Id]
	s.Require().True(exists)
	s.requireTaskTreeViewEqual(expected, taskSubtree)
}

func (s *TaskSchedulerDebugRpcTestSuite) requireTaskTreeViewEqual(
	expected *types.TaskEntry, actual *public.TaskTreeView,
) {
	s.T().Helper()
	s.Require().NotNil(actual)

	assertions := s.commonTaskAssertions(expected, &actual.TaskViewCommon)

	s.requireAllPass(assertions, expected.Task.Id)
}

func (s *TaskSchedulerDebugRpcTestSuite) commonTaskAssertions(
	expected *types.TaskEntry, actual *public.TaskViewCommon,
) []bool {
	now := s.clock.Now()

	return []bool{
		s.Equal(expected.Task.Id, actual.Id),
		s.Equal(expected.Task.TaskType, actual.Type),
		s.Equal(expected.Task.CircuitType, actual.CircuitType),

		s.Equal(expected.ExecutionTime(now), actual.ExecutionTime),
		s.Equal(expected.Owner, actual.Owner),
		s.Equal(expected.Status, actual.Status),
	}
}

func (s *TaskSchedulerDebugRpcTestSuite) requireAllPass(assertions []bool, id types.TaskId) {
	if slices.Contains(assertions, false) {
		s.FailNowf("", "assertion for task with id=%s failed", id.String())
	}
}
