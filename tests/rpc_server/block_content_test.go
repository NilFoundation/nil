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

	dm := &types.DeployMessage{
		ShardId: types.BaseShardId,
		Data:    hexutil.FromHex("6009600c60003960096000f3600054600101600055"),
	}

	data, err := dm.MarshalSSZ()
	suite.Require().NoError(err)

	m := &types.Message{
		From: types.GenerateRandomAddress(types.BaseShardId),
		Data: data,
	}
	suite.Require().NoError(m.Sign(key))

	suite.sendRawTransaction(m)

	suite.Eventually(func() bool {
		res := suite.getBlockByNumber(types.BaseShardId, "latest", true)
		return len(res.Messages) > 0
	}, 6*time.Second, 100*time.Millisecond)

	latestRes := suite.getBlockByNumber(types.BaseShardId, "latest", true)
	suite.Require().NotNil(latestRes.Hash)
	suite.Require().Len(latestRes.Messages, 1)

	msg, ok := latestRes.Messages[0].(map[string]any)
	suite.Require().True(ok)
	suite.Require().Equal(msg["signature"], m.Signature.Hex())
}
