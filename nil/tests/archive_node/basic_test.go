package tests

import (
	"testing"

	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/stretchr/testify/suite"
)

// SuiteArchiveNodeBasic contains tests that work with a single instance of the archive node.
// If a test does not require special start/stop logic, it should be added to this suite.
type SuiteArchiveNodeBasic struct {
	SuiteArchiveNode
}

func (s *SuiteArchiveNodeBasic) SetupSuite() {
	s.SuiteArchiveNode.SetupSuite()

	s.startCluster(oldProtocolVersion)
	s.startArchiveNode(s.newDb())
}

func (s *SuiteArchiveNodeBasic) TestCheckBlocksGeneraion() {
	s.checkBlocksGeneration()
}

func (s *SuiteArchiveNodeBasic) TestGetFaucetBalance() {
	value, err := s.DefaultClient.GetBalance(s.Context, types.FaucetAddress, "latest")
	s.Require().NoError(err)
	s.NotZero(value.Uint64())
}

func TestArchiveNodeWithBootstrapPeers(t *testing.T) {
	t.Parallel()

	suite.Run(t, &SuiteArchiveNodeBasic{SuiteArchiveNode{
		withBootstrapPeers: true,
		port:               10005,
	}})
}

func TestArchiveNodeWithoutBootstrapPeers(t *testing.T) {
	t.Parallel()

	suite.Run(t, &SuiteArchiveNodeBasic{SuiteArchiveNode{
		withBootstrapPeers: false,
		port:               10015,
	}})
}
