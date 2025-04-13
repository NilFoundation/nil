package tests

import (
	"testing"
	"time"

	"github.com/NilFoundation/nil/nil/common/concurrent"
	"github.com/NilFoundation/nil/nil/internal/collate"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/stretchr/testify/suite"
)

type dbWrapper struct {
	db.DB

	Dropped bool
}

func (d *dbWrapper) Close() {
}

func (d *dbWrapper) DropAll() error {
	d.Dropped = true
	return d.DB.DropAll()
}

type SuiteArchiveNodeComplex struct {
	SuiteArchiveNode
}

func (s *SuiteArchiveNodeComplex) TearDownTest() {
	s.stopArchiveNode()
	s.Cancel()
}

func (s *SuiteArchiveNodeComplex) newDbWrapper() *dbWrapper {
	s.T().Helper()

	return &dbWrapper{DB: s.newDb()}
}

func (s *SuiteArchiveNodeComplex) TestRestarts() {
	database := s.newDbWrapper()

	s.Run("Start", func() {
		s.startCluster(oldProtocolVersion)
		s.startArchiveNode(database)
	})

	s.Run("Before", s.checkBlocksGeneration)

	s.Run("Restart", func() {
		s.stopArchiveNode()
		s.startArchiveNode(database)

		s.False(database.Dropped)
	})

	s.Run("AfterRestart", s.checkBlocksGeneration)

	s.Run("ClusterReset", func() {
		s.stopArchiveNode()
		s.Cancel()
		s.startCluster(oldProtocolVersion)
		s.startArchiveNode(database)

		s.True(database.Dropped)
	})

	s.Run("AfterClusterReset", s.checkBlocksGeneration)
}

func (s *SuiteArchiveNodeComplex) TestProtocolUpdate() {
	database := s.newDb()

	s.Run("Start", func() {
		s.startCluster(oldProtocolVersion)
		s.startArchiveNode(database)
	})

	s.Run("Before", s.checkBlocksGeneration)

	s.Run("RestartClusterWithNewProtocolVersion", func() {
		s.stopArchiveNode()
		s.Cancel()
		s.startCluster(newProtocolVersion)

		_, _, rc := s.runArchiveNode(database)

		select {
		case err := <-rc:
			var executionError *concurrent.ExecutionError
			s.Require().ErrorAs(err, &executionError)
			var protocolVersionMismatchErr *collate.ProtocolVersionMismatchError
			s.Require().ErrorAs(executionError.Err, &protocolVersionMismatchErr)
			s.Require().Equal(oldProtocolVersion, protocolVersionMismatchErr.LocalVersion)
			s.Require().Equal(newProtocolVersion, protocolVersionMismatchErr.RemoteVersion)

		case <-time.After(10 * time.Second):
			s.Fail("The archive node did not react to the unsupported protocol")
		}
	})
}

func TestArchiveNodeComplex(t *testing.T) {
	t.Parallel()

	suite.Run(t, &SuiteArchiveNodeComplex{SuiteArchiveNode{
		withBootstrapPeers: true,
		port:               10025,
	}})
}
