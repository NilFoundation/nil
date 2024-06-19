package rpctest

import (
	"time"

	"github.com/NilFoundation/nil/contracts"
	"github.com/NilFoundation/nil/core/types"
)

func (suite *SuiteRpc) TestRpcBlockContent() {
	// Deploy message
	code, err := contracts.GetCode("tests/Counter")
	suite.Require().NoError(err)
	m := suite.createMessageForDeploy(code, types.BaseShardId)

	suite.sendRawTransaction(m)

	suite.Eventually(func() bool {
		res, err := suite.client.GetBlock(types.BaseShardId, "latest", false)
		suite.Require().NoError(err)

		return len(res.Messages) > 0
	}, 6*time.Second, 50*time.Millisecond)

	latestRes, err := suite.client.GetBlock(types.BaseShardId, "latest", true)
	suite.Require().NoError(err)

	suite.Require().NotNil(latestRes.Hash)
	suite.Require().Len(latestRes.Messages, 1)

	_, ok := latestRes.Messages[0].(map[string]any)
	suite.Require().True(ok)
}
