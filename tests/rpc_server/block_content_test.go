package rpctest

import (
	"encoding/hex"
	"time"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/hexutil"
	"github.com/NilFoundation/nil/core/crypto"
	"github.com/NilFoundation/nil/core/types"
)

func (suite *SuiteRpc) TestRpcBlockContent() {
	// Deploy message
	key, err := crypto.GenerateKey()
	suite.Require().NoError(err)

	dm := types.DeployMessage{
		ShardId: uint32(types.MasterShardId),
		Data:    hexutil.FromHex("6009600c60003960096000f3600054600101600055"),
	}

	data, err := dm.MarshalSSZ()
	suite.Require().NoError(err)

	m := types.Message{
		From: common.GenerateRandomAddress(uint32(types.MasterShardId)),
		Data: data,
	}

	suite.Require().NoError(m.Sign(key))

	mData, err := m.MarshalSSZ()
	suite.Require().NoError(err)

	request := &Request{
		Jsonrpc: "2.0",
		Method:  sendRawTransaction,
		Params:  []any{"0x" + hex.EncodeToString(mData)},
		Id:      1,
	}

	resp, err := makeRequest[common.Hash](suite.port, request)
	suite.Require().NoError(err)
	suite.Require().Nil(resp.Error["code"])
	suite.Equal(m.Hash(), resp.Result)

	// Check the latest block content
	request = &Request{
		Jsonrpc: "2.0",
		Method:  getBlockByNumber,
		Params:  []any{types.MasterShardId, "latest", true},
		Id:      1,
	}

	suite.Eventually(func() bool {
		latestResp, err := makeRequest[map[string]any](suite.port, request)
		suite.Require().NoError(err)
		msgs, ok := latestResp.Result["messages"].([]any)
		if !ok {
			return false
		}
		return len(msgs) == 1
	}, 6*time.Second, 100*time.Millisecond)

	latestResp, err := makeRequest[map[string]any](suite.port, request)
	suite.Require().NoError(err)
	suite.Require().Nil(latestResp.Error["code"])
	suite.Require().NotNil(latestResp.Result["hash"])

	suite.Require().Len(latestResp.Result["messages"], 1)

	msgs, ok := latestResp.Result["messages"].([]any)
	suite.Require().True(ok)
	msg, ok := msgs[0].(map[string]any)
	suite.Require().True(ok)
	suite.Require().Equal(msg["signature"], m.Signature.Hex())
}
