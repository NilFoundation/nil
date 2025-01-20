package storage

import (
	"context"
	"errors"
	"testing"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/testaide"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	"github.com/stretchr/testify/suite"
)

type TaskResultStorageSuite struct {
	suite.Suite

	ctx    context.Context
	cancel context.CancelFunc

	database db.DB
	storage  TaskResultStorage
}

func TestTaskResultStorageSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(TaskResultStorageSuite))
}

func (s *TaskResultStorageSuite) SetupSuite() {
	s.ctx, s.cancel = context.WithCancel(context.Background())

	database, err := db.NewBadgerDbInMemory()
	s.Require().NoError(err)
	s.database = database

	logger := logging.NewLogger("task_result_storage_test")
	s.storage = NewTaskResultStorage(database, logger)
}

func (s *TaskResultStorageSuite) TearDownSuite() {
	s.cancel()
}

func (s *TaskResultStorageSuite) TearDownTest() {
	err := s.database.DropAll()
	s.Require().NoError(err, "failed to clear database in TearDownTest")
}

func (s *TaskResultStorageSuite) Test_Put_Same_Task_Result_N_Times() {
	result := types.NewSuccessProviderTaskResult(
		types.NewTaskId(),
		testaide.RandomExecutorId(),
		types.TaskOutputArtifacts{types.AggregatedProof: "agg-proof.1.1.0xAABC"},
		testaide.RandomTaskResultData(),
	)

	for range 3 {
		err := s.storage.Put(s.ctx, result)
		s.Require().NoError(err)
	}

	resultFromStorage, err := s.storage.TryGetPending(s.ctx)
	s.Require().NoError(err)
	s.Require().Equal(result, resultFromStorage)
}

func (s *TaskResultStorageSuite) Test_Delete_Same_Task_Result_N_Times() {
	result := types.NewFailureProviderTaskResult(
		types.NewTaskId(),
		testaide.RandomExecutorId(),
		errors.New("something went wrong"),
	)

	err := s.storage.Put(s.ctx, result)
	s.Require().NoError(err)

	for range 3 {
		err := s.storage.Delete(s.ctx, result.TaskId)
		s.Require().NoError(err)

		nextRes, err := s.storage.TryGetPending(s.ctx)
		s.Require().NoError(err)
		s.Require().Nil(nextRes)
	}
}

func (s *TaskResultStorageSuite) Test_Put_Get_Delete_Results() {
	results := newTaskResults()

	for _, result := range results {
		err := s.storage.Put(s.ctx, result)
		s.Require().NoError(err)
	}

	resultsFromStorage := make([]*types.TaskResult, 0, len(results))

	for range results {
		resultFromStorage, err := s.storage.TryGetPending(s.ctx)
		s.Require().NoError(err)
		s.Require().NotNil(resultFromStorage)

		resultsFromStorage = append(resultsFromStorage, resultFromStorage)

		err = s.storage.Delete(s.ctx, resultFromStorage.TaskId)
		s.Require().NoError(err)
	}

	s.Require().ElementsMatch(results, resultsFromStorage)

	// There are no results left in the storage
	resultFromStorage, err := s.storage.TryGetPending(s.ctx)
	s.Require().NoError(err)
	s.Require().Nil(resultFromStorage)
}

func newTaskResults() []*types.TaskResult {
	return []*types.TaskResult{
		types.NewSuccessProviderTaskResult(
			types.NewTaskId(),
			testaide.RandomExecutorId(),
			types.TaskOutputArtifacts{types.AggregatedProof: "agg-proof.1.1.0xAABC"},
			testaide.RandomTaskResultData(),
		),
		types.NewFailureProviderTaskResult(
			types.NewTaskId(),
			testaide.RandomExecutorId(),
			errors.New("something went wrong"),
		),
		types.NewSuccessProverTaskResult(
			types.NewTaskId(),
			testaide.RandomExecutorId(),
			types.TaskOutputArtifacts{
				types.PartialProofChallenges:     "challenge.1.1.0xAABC",
				types.AssignmentTableDescription: "assignment_table_description.1.1.0xAABC",
				types.PartialProof:               "proof.1.1.0xAABC",
				types.CommitmentState:            "commitment_state.1.1.0xAABC",
			},
			testaide.RandomTaskResultData(),
		),
		types.NewFailureProverTaskResult(
			types.NewTaskId(),
			testaide.RandomExecutorId(),
			errors.New("prover failed to handle task"),
		),
	}
}
