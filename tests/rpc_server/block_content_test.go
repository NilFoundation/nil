package rpctest

import (
	"time"

	"github.com/NilFoundation/nil/common/hexutil"
	"github.com/NilFoundation/nil/core/crypto"
	"github.com/NilFoundation/nil/core/types"
)

func (suite *SuiteRpc) TestRpcBlockContent() {
	// Deploy message
	key, err := crypto.GenerateKey()
	suite.Require().NoError(err)

	from := types.GenerateRandomAddress(types.BaseShardId)
	code := hexutil.FromHex("6009600c60003960096000f3600054600101600055")
	m := suite.createMessageForDeploy(from, 0, code, types.BaseShardId, 10000)

	suite.Require().NoError(m.Sign(key))

	suite.sendRawTransaction(m)

	suite.Eventually(func() bool {
		res, err := suite.client.GetBlock(types.BaseShardId, "latest", false)
		suite.Require().NoError(err)

		return len(res.Messages) > 0
	}, 6*time.Second, 100*time.Millisecond)

	latestRes, err := suite.client.GetBlock(types.BaseShardId, "latest", true)
	suite.Require().NoError(err)

	suite.Require().NotNil(latestRes.Hash)
	suite.Require().Len(latestRes.Messages, 1)

	msg, ok := latestRes.Messages[0].(map[string]any)
	suite.Require().True(ok)
	suite.Require().Equal(msg["signature"], m.Signature.Hex())
}
