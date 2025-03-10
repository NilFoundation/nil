package governance

import (
	"testing"

	"github.com/NilFoundation/nil/nil/internal/collate"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/nilservice"
	"github.com/NilFoundation/nil/nil/services/rpc"
	"github.com/NilFoundation/nil/nil/services/rpc/transport"
	"github.com/NilFoundation/nil/nil/tests"
	"github.com/stretchr/testify/suite"
)

const numShards = 4

type RollbackSuite struct {
	tests.RpcSuite
}

func (s *RollbackSuite) SetupTest() {
	s.Start(&nilservice.Config{
		NShards:              numShards,
		HttpUrl:              rpc.GetSockPath(s.T()),
		CollatorTickPeriodMs: 300,
		RunMode:              nilservice.CollatorsOnlyRunMode,
	})
}

func (s *RollbackSuite) TearDownTest() {
	s.Cancel()
}

func (s *RollbackSuite) TestSendRollbackTx() {
	params := &execution.RollbackParams{
		Version:     1,
		Counter:     0,
		PatchLevel:  1,
		MainBlockId: 2,
		ReplayDepth: 3,
		SearchDepth: 4,
	}

	calldata, err := collate.CreateRollbackCalldata(params)
	s.Require().NoError(err)

	receipt := s.SendExternalTransaction(calldata, types.GovernanceAddress)
	s.Require().NotNil(receipt)
	s.True(receipt.Success)

	// Check that the patchLevel has been updated
	block, err := s.Client.GetBlock(s.Context, types.MainShardId, transport.BlockNumber(receipt.BlockNumber), false)
	s.Require().NoError(err)
	s.Equal(uint32(1), block.PatchLevel)
}

func TestRollback(t *testing.T) {
	t.Parallel()

	suite.Run(t, &RollbackSuite{})
}
