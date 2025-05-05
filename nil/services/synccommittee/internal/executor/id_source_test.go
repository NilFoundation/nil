package executor

import (
	"context"
	"testing"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/storage"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	"github.com/stretchr/testify/suite"
	"golang.org/x/sync/errgroup"
)

func TestIdSourceSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(IdSourceTestSuite))
}

type IdSourceTestSuite struct {
	suite.Suite
	context context.Context
	cancel  context.CancelFunc

	database  db.DB
	idStorage IdStorage
}

func (s *IdSourceTestSuite) SetupSuite() {
	s.context, s.cancel = context.WithCancel(context.Background())
	logger := logging.NewLogger("id_source_test")

	var err error
	s.database, err = db.NewBadgerDbInMemory()
	s.Require().NoError(err)
	s.idStorage = storage.NewExecutorIdStorage(s.database, logger)
}

func (s *IdSourceTestSuite) TearDownSuite() {
	s.cancel()
}

func (s *IdSourceTestSuite) TearDownTest() {
	err := s.database.DropAll()
	s.Require().NoError(err, "failed to clear database in TearDownTest")
}

func (s *IdSourceTestSuite) Test_Generate_New_Id_And_Store() {
	for _, testCase := range s.testCases() {
		s.Run(testCase.name, func() {
			s.testGenerateNewAndCache(testCase.idSource)
		})
	}
}

func (s *IdSourceTestSuite) testGenerateNewAndCache(idSource IdSource) {
	s.T().Helper()

	newId, err := idSource.GetCurrentId(s.context)
	s.Require().NoError(err)
	s.Require().NotNil(newId)
	s.Require().NotEqual(types.UnknownExecutorId, *newId)

	existingId, err := idSource.GetCurrentId(s.context)
	s.Require().NoError(err)
	s.Require().Equal(newId, existingId)
}

func (s *IdSourceTestSuite) Test_Concurrent_Access() {
	for _, testCase := range s.testCases() {
		s.Run(testCase.name, func() {
			s.testConcurrentAccess(testCase.idSource)
		})
	}
}

func (s *IdSourceTestSuite) testConcurrentAccess(idSource IdSource) {
	s.T().Helper()

	const goroutines = 10
	ids := make(chan *types.TaskExecutorId, goroutines)

	var group errgroup.Group
	for range goroutines {
		group.Go(func() error {
			id, err := idSource.GetCurrentId(s.context)
			if err != nil {
				return err
			}
			ids <- id
			return nil
		})
	}

	err := group.Wait()
	s.Require().NoError(err)

	expectedId := <-ids
	s.Require().NotNil(expectedId)
	s.Require().NotEqual(types.UnknownExecutorId, *expectedId)

	for range goroutines - 1 {
		id := <-ids
		s.Require().NotNil(id)
		s.Require().Equal(*expectedId, *id)
	}
}

func (s *IdSourceTestSuite) Test_PersistentSource_RetrievesExistingId() {
	idSource := NewPersistentIdSource(s.idStorage)

	expectedId := types.NewRandomExecutorId()
	_, err := s.idStorage.GetOrAdd(s.context, func() types.TaskExecutorId {
		return expectedId
	})
	s.Require().NoError(err)

	actualId, err := idSource.GetCurrentId(s.context)
	s.Require().NoError(err)
	s.Require().NotNil(actualId)
	s.Require().Equal(expectedId, *actualId)
}

func (s *IdSourceTestSuite) testCases() []struct {
	name     string
	idSource IdSource
} {
	s.T().Helper()

	return []struct {
		name     string
		idSource IdSource
	}{
		{"In_Memory_Source", NewInMemoryIdSource()},
		{"Persistent_Source", NewPersistentIdSource(s.idStorage)},
	}
}
