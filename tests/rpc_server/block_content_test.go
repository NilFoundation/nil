package rpctest

import (
	"time"

	"github.com/NilFoundation/nil/contracts"
	"github.com/NilFoundation/nil/core/execution"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/rpc/jsonrpc"
)

func (s *SuiteRpc) TestRpcBlockContent() {
	// Deploy message
	hash, _, err := s.client.DeployContract(types.BaseShardId, types.MainWalletAddress,
		contracts.CounterDeployPayload(s.T()), nil,
		execution.MainPrivateKey)
	s.Require().NoError(err)

	var block *jsonrpc.RPCBlock
	s.Eventually(func() bool {
		var err error
		block, err = s.client.GetBlock(types.BaseShardId, "latest", false)
		s.Require().NoError(err)

		return len(block.Messages) > 0
	}, 6*time.Second, 50*time.Millisecond)

	block, err = s.client.GetBlock(types.BaseShardId, block.Hash, true)
	s.Require().NoError(err)

	s.Require().NotNil(block.Hash)
	s.Require().Len(block.Messages, 1)

	msg, ok := block.Messages[0].(map[string]any)
	s.Require().True(ok)
	s.Equal(hash.Hex(), msg["hash"])
}
