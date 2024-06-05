package rpctest

import (
	"context"
	"encoding/hex"
	"strconv"
	"testing"
	"time"

	"github.com/NilFoundation/nil/cmd/nil/nilservice"
	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/hexutil"
	"github.com/NilFoundation/nil/core/collate"
	"github.com/NilFoundation/nil/core/crypto"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/execution"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/rpc/jsonrpc"
	"github.com/NilFoundation/nil/tools/solc"
	"github.com/stretchr/testify/suite"
)

type SuiteRpc struct {
	suite.Suite
	port    int
	context context.Context
	cancel  context.CancelFunc
}

func (suite *SuiteRpc) SetupSuite() {
	suite.context, suite.cancel = context.WithCancel(context.Background())

	badger, err := db.NewBadgerDbInMemory()
	suite.Require().NoError(err)

	suite.port = 8531
	cfg := &nilservice.Config{
		NShards:  5,
		HttpPort: suite.port,
		Topology: collate.NeighbouringShardTopologyId,
	}
	go nilservice.Run(suite.context, cfg, badger)
	time.Sleep(time.Second) // To be sure that server is started
}

func (suite *SuiteRpc) TearDownSuite() {
	suite.cancel()
}

func (suite *SuiteRpc) makeGenericRequest(method string, params ...any) map[string]any {
	suite.T().Helper()

	request := NewRequest(method, params...)
	resp, err := makeRequest[map[string]any](suite.port, request)
	suite.Require().NoError(err)
	suite.Require().NotNil(resp)
	suite.Require().Nil(resp.Error)
	return resp.Result
}

func (suite *SuiteRpc) sendRawTransaction(m *types.Message) {
	suite.T().Helper()

	data, err := m.MarshalSSZ()
	suite.Require().NoError(err)

	request := NewRequest(sendRawTransaction, "0x"+hex.EncodeToString(data))
	resp, err := makeRequest[common.Hash](suite.port, request)
	suite.Require().NoError(err)
	suite.Require().NotNil(resp)
	suite.Require().Nil(resp.Error)
	suite.Require().Equal(m.Hash(), resp.Result)
}

func (suite *SuiteRpc) getBlockByNumber(shardId types.ShardId, blk string, b bool) *jsonrpc.RPCBlock {
	suite.T().Helper()

	request := NewRequest(getBlockByNumber, shardId, blk, b)
	resp, err := makeRequest[*jsonrpc.RPCBlock](suite.port, request)
	suite.Require().NoError(err)
	suite.Require().NotNil(resp)
	suite.Require().Nil(resp.Error)
	suite.Require().NotNil(resp.Result)
	return resp.Result
}

//nolint:unparam
func (suite *SuiteRpc) getTransactionCount(addr types.Address, blk string) uint64 {
	suite.T().Helper()

	request := NewRequest(getTransactionCount, addr, blk)
	resp, err := makeRequest[string](suite.port, request)
	suite.Require().NoError(err)
	suite.Require().NotNil(resp)
	suite.Require().Nil(resp.Error)
	suite.Require().NotNil(resp.Result)

	res, err := strconv.ParseUint(resp.Result, 0, 64)
	suite.Require().NoError(err)
	return res
}

func (suite *SuiteRpc) waitForReceiptOnShard(shardId types.ShardId, addr types.Address, msg *types.Message) {
	suite.T().Helper()

	request := NewRequest(getInMessageReceipt, shardId, msg.Hash())

	var respReceipt *Response[*jsonrpc.RPCReceipt]
	var err error
	suite.Require().Eventually(func() bool {
		respReceipt, err = makeRequest[*jsonrpc.RPCReceipt](suite.port, request)
		suite.Require().NoError(err)
		suite.Require().Nil(respReceipt.Error)
		return respReceipt.Result != nil
	}, 6*time.Hour, 200*time.Millisecond)

	suite.True(respReceipt.Result.Success)
	suite.Equal(types.MessageIndex(0), respReceipt.Result.MsgIndex) // now in all test cases it's the first msg in block
	suite.Equal(msg.Hash(), respReceipt.Result.MsgHash)
	suite.Equal(addr, respReceipt.Result.ContractAddress)
}

func (suite *SuiteRpc) waitForReceipt(addr types.Address, msg *types.Message) {
	suite.T().Helper()
	suite.waitForReceiptOnShard(types.BaseShardId, addr, msg)
}

func (suite *SuiteRpc) deployContract(from types.Address, code types.Code, seqno uint64) types.Address {
	suite.T().Helper()

	shardId := from.ShardId()

	dm := &types.DeployMessage{
		ShardId: from.ShardId(),
		Code:    code,
	}
	data, err := dm.MarshalSSZ()
	suite.Require().NoError(err)

	msg := &types.Message{
		Seqno: seqno,
		Data:  data,
		From:  from,
	}
	suite.Require().NoError(msg.Sign(execution.MainPrivateKey))

	// create contract
	suite.sendRawTransaction(msg)

	// wait for receipt
	addr := types.CreateAddress(shardId, msg.From, msg.Seqno)
	suite.waitForReceiptOnShard(shardId, addr, msg)
	return addr
}

func (suite *SuiteRpc) TestRpcBasic() {
	const someRandomMissingBlock = "0x6804117de2f3e6ee32953e78ced1db7b20214e0d8c745a03b8fecf7cc8ee76ef"

	makeReq := func(method string, params ...any) any {
		request := NewRequest(method, params...)
		resp, err := makeRequest[any](suite.port, request)
		suite.Require().NoError(err)
		suite.Require().NotNil(resp)
		suite.Require().Nil(resp.Error)
		return resp.Result
	}

	res := makeReq(getBlockByNumber, types.BaseShardId, "0x1b4", false)
	suite.Require().Nil(res)

	res = makeReq(getBlockTransactionCountByNumber, types.BaseShardId, "earliest")
	suite.Equal("0x0", res)

	res = makeReq(getBlockTransactionCountByHash, types.BaseShardId, someRandomMissingBlock)
	suite.Equal("0x0", res)

	res = makeReq(getBlockByHash, types.BaseShardId, someRandomMissingBlock, false)
	suite.Require().Nil(res)

	res = makeReq(getBlockByNumber, types.BaseShardId, "earliest", false)
	suite.Require().NotNil(res)

	latest := suite.makeGenericRequest(getBlockByNumber, types.BaseShardId, "latest", false)

	res = makeReq(getBlockByHash, types.BaseShardId, latest["hash"], false)
	suite.Require().Equal(latest, res)

	res = makeReq(getInMessageByHash, types.BaseShardId, someRandomMissingBlock)
	suite.Require().Nil(res)
}

func (suite *SuiteRpc) TestRpcContract() {
	pub := crypto.CompressPubkey(&execution.MainPrivateKey.PublicKey)
	from := types.PubkeyBytesToAddress(types.BaseShardId, pub)

	seqno := suite.getTransactionCount(from, "latest")

	contracts, err := solc.CompileSource("./contracts/increment.sol")
	suite.Require().NoError(err)
	contractCode := hexutil.FromHex(contracts["Incrementer"].Code)

	dm := &types.DeployMessage{
		ShardId: types.BaseShardId,
		Code:    contractCode,
	}
	data, err := dm.MarshalSSZ()
	suite.Require().NoError(err)

	m := &types.Message{
		Seqno: seqno,
		Data:  data,
		From:  from,
	}
	suite.Require().NoError(m.Sign(execution.MainPrivateKey))

	// create contract
	suite.sendRawTransaction(m)

	suite.Eventually(func() bool {
		seqno := suite.getTransactionCount(from, "latest")
		return seqno == m.Seqno+1
	}, 6*time.Second, 200*time.Millisecond)

	addr := types.CreateAddress(types.BaseShardId, m.From, m.Seqno)

	suite.waitForReceipt(addr, m)

	// now call (= send a message to) created contract. as a result, it should also create a new contract
	seqno = suite.getTransactionCount(from, "latest")

	abi := solc.ExtractABI(contracts["Incrementer"])
	calldata, err := abi.Pack("increment_and_send_msg")
	suite.Require().NoError(err)
	m = &types.Message{
		Seqno: seqno,
		From:  from,
		To:    addr,
		Data:  calldata,
	}
	suite.Require().NoError(m.Sign(execution.MainPrivateKey))

	suite.sendRawTransaction(m)

	suite.waitForReceipt(addr, m)
}

func (suite *SuiteRpc) TestRpcContractSendMessage() {
	pub := crypto.CompressPubkey(&execution.MainPrivateKey.PublicKey)
	from := types.PubkeyBytesToAddress(types.BaseShardId, pub)

	nbShardId := types.ShardId(4)
	nbFrom := types.PubkeyBytesToAddress(nbShardId, pub)

	seqno := suite.getTransactionCount(from, "latest")
	nbSeqno := suite.getTransactionCount(nbFrom, "latest")

	// Deploy contract on neighbour shard
	code := hexutil.FromHex("6009600c60003960096000f3600054600101600055")
	nbAddr := suite.deployContract(nbFrom, code, nbSeqno)

	// Create internal message to the neighbouring shard
	mSend := &types.Message{
		Seqno:    nbSeqno,
		From:     from,
		To:       nbAddr,
		Internal: true,
	}
	suite.Require().NoError(mSend.Sign(execution.MainPrivateKey))
	mSendData, err := mSend.MarshalSSZ()
	suite.Require().NoError(err)

	// call SendMessage precompiled contract that executes sends message to neighbour shard
	sendMessageAddr := types.BytesToAddress([]byte{0x06}) // sendMessagePrecompiledContract
	m := &types.Message{
		Seqno: seqno,
		From:  from,
		To:    sendMessageAddr,
		Data:  mSendData,
	}
	suite.Require().NoError(m.Sign(execution.MainPrivateKey))

	suite.sendRawTransaction(m)

	suite.waitForReceipt(sendMessageAddr, m)

	// This message is handled as an outgoing one, it is received by the neighbour shard
	suite.waitForReceiptOnShard(nbShardId, nbAddr, mSend)
}

func (suite *SuiteRpc) TestRpcApiModules() {
	res := suite.makeGenericRequest("rpc_modules")
	suite.Equal("1.0", res["eth"])
	suite.Equal("1.0", res["rpc"])
}

func (suite *SuiteRpc) TestRpcError() {
	check := func(code int, msg, method string, params ...any) {
		request := NewRequest(method, params...)
		resp, err := makeRequest[any](suite.port, request)
		suite.Require().NoError(err)
		suite.Require().NotNil(resp.Error)
		suite.InEpsilon(float64(code), resp.Error.Code, 0.1)
		suite.Equal(msg, resp.Error.Message)
	}

	check(-32601, "the method eth_doesntExists does not exist/is not available",
		"eth_doesntExists")

	check(-32602, "missing value for required argument 0",
		getBlockByNumber)

	check(-32602, "invalid argument 0: json: cannot unmarshal number 1099511627776 into Go value of type uint32",
		getBlockByNumber, 1<<40)

	check(-32602, "missing value for required argument 1",
		getBlockByNumber, types.BaseShardId)

	check(-32000, "invalid argument 1: hex string of odd length",
		getBlockByHash, types.BaseShardId, "0x1b4", false)

	check(-32000, "invalid argument 1: hex string without 0x prefix",
		getBlockByHash, types.BaseShardId, "latest")
}

func (suite *SuiteRpc) TestRpcDebugModules() {
	res := suite.makeGenericRequest("debug_getBlockByNumber", types.BaseShardId, "latest", false)

	suite.Require().Contains(res, "number")
	suite.Require().Contains(res, "hash")
	suite.Require().Contains(res, "content")

	sliceContent, ok := res["content"].(string)
	suite.Require().True(ok)
	// check if the string starts with 0x prefix
	suite.Require().Equal("0x", sliceContent[:2])
	// print resp to see the result
	suite.T().Logf("resp: %v", res)

	fullRes := suite.makeGenericRequest("debug_getBlockByNumber", types.BaseShardId, "latest", true)
	suite.Require().Contains(fullRes, "content")
	suite.Require().Contains(fullRes, "messages")
	suite.Require().Contains(fullRes, "receipts")
}

func TestSuiteRpc(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteRpc))
}
