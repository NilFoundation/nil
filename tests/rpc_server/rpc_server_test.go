package rpctest

import (
	"context"
	"encoding/hex"
	"testing"
	"time"

	"github.com/NilFoundation/nil/cmd/nil/nilservice"
	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/core/crypto"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/shardchain"
	"github.com/NilFoundation/nil/core/types"
	"github.com/stretchr/testify/suite"
)

type SuiteRpc struct {
	suite.Suite
	context context.Context
	cancel  context.CancelFunc
}

func (suite *SuiteRpc) SetupSuite() {
	suite.context, suite.cancel = context.WithCancel(context.Background())

	dbOpts := db.BadgerDBOptions{Path: suite.T().TempDir() + "/test.db", DiscardRatio: 0.5, GcFrequency: time.Hour, AllowDrop: false}
	badger, err := db.NewBadgerDb(dbOpts.Path)
	suite.Require().NoError(err)

	go nilservice.Run(suite.context, 2, badger, dbOpts)
	time.Sleep(time.Second) // To be sure that server is started
}

func (suite *SuiteRpc) TearDownSuite() {
	suite.cancel()
}

func (suite *SuiteRpc) TestRpcBasic() {
	const someRandomMissingBlock = "0x6804117de2f3e6ee32953e78ced1db7b20214e0d8c745a03b8fecf7cc8ee76ef"
	request := Request{
		Jsonrpc: "2.0",
		Method:  getBlockByNumber,
		Params:  []any{types.MasterShardId, "0x1b4", false},
		Id:      1,
	}

	resp, err := makeRequest[any](&request)
	suite.Require().NoError(err)
	suite.Require().Nil(resp.Error["code"])
	suite.Require().Nil(resp.Result)

	request.Method = "eth_getBlockTransactionCountByNumber"
	request.Params = []any{types.MasterShardId, "0x1b4"}
	resp, err = makeRequest[any](&request)
	suite.Require().NoError(err)
	suite.InEpsilon(float64(-32000), resp.Error["code"], 0)
	suite.Equal("not implemented", resp.Error["message"])

	request.Method = "eth_getBlockTransactionCountByHash"
	request.Params = []any{types.MasterShardId, someRandomMissingBlock}
	resp, err = makeRequest[any](&request)
	suite.Require().NoError(err)
	suite.InEpsilon(float64(-32000), resp.Error["code"], 0)
	suite.Equal("not implemented", resp.Error["message"])

	request.Method = getBlockByHash
	request.Params = []any{types.MasterShardId, someRandomMissingBlock, false}
	resp, err = makeRequest[any](&request)
	suite.Require().NoError(err)
	suite.Require().Nil(resp.Error["code"])
	suite.Require().Nil(resp.Result)

	request.Method = getBlockByNumber
	request.Params = []any{types.MasterShardId, "earliest", false}
	resp, err = makeRequest[any](&request)
	suite.Require().NoError(err)
	suite.Require().Nil(resp.Error)
	suite.Require().NotNil(resp.Result)

	request.Method = getBlockByNumber
	request.Params = []any{types.MasterShardId, "latest", false}
	latestResp, err := makeRequest[map[string]any](&request)
	suite.Require().NoError(err)
	suite.Require().Nil(latestResp.Error["code"])
	suite.Require().NotNil(latestResp.Result["hash"])

	request.Method = getBlockByHash
	hash, ok := latestResp.Result["hash"].(string)
	suite.Require().True(ok)
	request.Params = []any{types.MasterShardId, hash, false}
	resp, err = makeRequest[any](&request)
	suite.Require().NoError(err)
	suite.Require().Nil(resp.Error["code"])
	suite.Require().Equal(latestResp.Result, resp.Result)

	request.Method = getInMessageByHash
	request.Params = []any{types.MasterShardId, someRandomMissingBlock}
	resp, err = makeRequest[any](&request)
	suite.Require().NoError(err)
	suite.Require().Nil(resp.Error["code"])
	suite.Require().Nil(resp.Result)
}

func (suite *SuiteRpc) TestRpcContract() {
	pub := crypto.CompressPubkey(&shardchain.MainPrivateKey.PublicKey)
	from := common.PubkeyBytesToAddress(uint32(types.MasterShardId), pub)

	seqno1, err := transactionCount(types.MasterShardId, from, "latest")
	suite.Require().NoError(err)

	dm := &types.DeployMessage{
		ShardId: uint32(types.MasterShardId),
		Code:    HardcodedContract,
	}
	data, err := dm.MarshalSSZ()
	suite.Require().NoError(err)

	m := &types.Message{
		Seqno: seqno1,
		Data:  data,
		From:  from,
	}
	suite.Require().NoError(m.Sign(shardchain.MainPrivateKey))

	msgHash := m.Hash()
	mData, err := m.MarshalSSZ()
	suite.Require().NoError(err)

	request := &Request{
		Jsonrpc: "2.0",
		Method:  sendRawTransaction,
		Params:  []any{"0x" + hex.EncodeToString(mData)},
		Id:      1,
	}

	// create contract
	resp, err := makeRequest[common.Hash](request)
	suite.Require().NoError(err)
	suite.Require().Nil(resp.Error["code"])
	suite.Equal(m.Hash(), resp.Result)

	suite.Eventually(func() bool {
		seqno, err := transactionCount(types.MasterShardId, from, "latest")
		suite.Require().NoError(err)
		return seqno == seqno1+1
	}, 6*time.Second, 200*time.Millisecond)

	addr := common.CreateAddress(uint32(types.MasterShardId), m.From, m.Seqno)
	request.Method = getInMessageReceipt
	request.Params = []any{types.MasterShardId, msgHash}

	var respReceipt *Response[*types.Receipt]
	suite.Require().Eventually(func() bool {
		respReceipt, err = makeRequest[*types.Receipt](request)
		suite.Require().NoError(err)
		suite.Require().Nil(resp.Error["code"])
		return respReceipt.Result != nil
	}, 6*time.Second, 200*time.Millisecond)
	suite.Equal(uint64(0), respReceipt.Result.MsgIndex)
	suite.Equal(m.Hash(), respReceipt.Result.MsgHash)
	suite.Equal(addr, respReceipt.Result.ContractAddress)

	// now call (= send message to) created contract. as a result it should also create new contract
	seqno, err := transactionCount(types.MasterShardId, from, "latest")
	suite.Require().NoError(err)
	m = &types.Message{
		Seqno: seqno,
		From:  from,
		To:    addr,
	}
	suite.Require().NoError(m.Sign(shardchain.MainPrivateKey))
	mData, err = m.MarshalSSZ()
	suite.Require().NoError(err)

	request.Method = sendRawTransaction
	request.Params = []any{"0x" + hex.EncodeToString(mData)}

	resp, err = makeRequest[common.Hash](request)
	suite.Require().NoError(err)
	suite.Require().Nil(resp.Error["code"])
	suite.Equal(m.Hash(), resp.Result)
	time.Sleep(2 * time.Second)

	// TODO: finish this test (after extending rpc and improving execution msg handling)
	// now it's only possible to see in logs that `SendCall precompiled was called` =)
}

func (suite *SuiteRpc) TestRpcApiModules() {
	request := Request{
		Jsonrpc: "2.0",
		Method:  "rpc_modules",
		Params:  []any{},
		Id:      1,
	}

	resp, err := makeRequest[map[string]any](&request)
	suite.Require().NoError(err)
	suite.Equal("1.0", resp.Result["eth"])
	suite.Equal("1.0", resp.Result["rpc"])
}

func (suite *SuiteRpc) TestRpcError() {
	request := Request{
		Jsonrpc: "2.0",
		Method:  "eth_doesntExists",
		Params:  []any{},
		Id:      1,
	}

	resp, err := makeRequest[any](&request)
	suite.Require().NoError(err)
	suite.InEpsilon(float64(-32601), resp.Error["code"], 0)
	suite.Equal("the method eth_doesntExists does not exist/is not available", resp.Error["message"])

	request = Request{
		Jsonrpc: "2.0",
		Method:  getBlockByNumber,
		Params:  []any{},
		Id:      1,
	}
	resp, err = makeRequest[any](&request)
	suite.Require().NoError(err)
	suite.InEpsilon(float64(-32602), resp.Error["code"], 0)
	suite.Equal("missing value for required argument 0", resp.Error["message"])

	request.Method = getBlockByNumber
	request.Params = []any{1 << 40}
	resp, err = makeRequest[any](&request)
	suite.Require().NoError(err)
	suite.InEpsilon(float64(-32602), resp.Error["code"], 0)
	suite.Equal(
		"invalid argument 0: json: cannot unmarshal number 1099511627776 into Go value of type uint32",
		resp.Error["message"])

	request.Method = getBlockByNumber
	request.Params = []any{types.MasterShardId}
	resp, err = makeRequest[any](&request)
	suite.Require().NoError(err)
	suite.InEpsilon(float64(-32602), resp.Error["code"], 0)
	suite.Equal("missing value for required argument 1", resp.Error["message"])

	request.Method = getBlockByHash
	request.Params = []any{types.MasterShardId, "0x1b4", false}
	resp, err = makeRequest[any](&request)
	suite.Require().NoError(err)
	suite.InEpsilon(float64(-32000), resp.Error["code"], 0.5)
	suite.Equal("invalid argument 1: hex string of odd length", resp.Error["message"])

	request.Method = getBlockByHash
	request.Params = []any{types.MasterShardId, "latest"}
	resp, err = makeRequest[any](&request)
	suite.Require().NoError(err)
	suite.InEpsilon(float64(-32000), resp.Error["code"], 0.5)
	suite.Equal("invalid argument 1: hex string without 0x prefix", resp.Error["message"])
}

func (suite *SuiteRpc) TestRpcDebugModules() {
	request := Request{
		Jsonrpc: "2.0",
		Method:  "debug_getBlockByNumber",
		Params:  []any{types.MasterShardId, "latest"},
	}

	resp, err := makeRequest[map[string]any](&request)
	suite.Require().NoError(err)

	suite.Require().Contains(resp.Result, "number")
	suite.Require().Contains(resp.Result, "hash")
	suite.Require().Contains(resp.Result, "content")

	sliceContent, ok := resp.Result["content"].(string)
	suite.Require().True(ok)
	// check if the string starts with 0x prefix
	suite.Require().Equal("0x", sliceContent[:2])
	// print resp to see the result
	suite.T().Logf("resp: %v", resp)
}

func TestSuiteRpc(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteRpc))
}
