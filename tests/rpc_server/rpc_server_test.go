package rpctest

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/NilFoundation/nil/client"
	rpc_client "github.com/NilFoundation/nil/client/rpc"
	"github.com/NilFoundation/nil/cmd/nil/nilservice"
	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/hexutil"
	"github.com/NilFoundation/nil/common/logging"
	"github.com/NilFoundation/nil/core/collate"
	"github.com/NilFoundation/nil/core/crypto"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/execution"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/rpc/jsonrpc"
	"github.com/NilFoundation/nil/rpc/transport"
	"github.com/NilFoundation/nil/tools/solc"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/stretchr/testify/suite"
)

type SuiteRpc struct {
	suite.Suite
	port    int
	context context.Context
	cancel  context.CancelFunc
	address types.Address
	client  client.Client
}

func (suite *SuiteRpc) SetupSuite() {
	logging.SetupGlobalLogger()
}

func (suite *SuiteRpc) SetupTest() {
	suite.context, suite.cancel = context.WithCancel(context.Background())

	badger, err := db.NewBadgerDbInMemory()
	suite.Require().NoError(err)

	suite.client = rpc_client.NewClient("http://127.0.0.1:8531/")

	suite.port = 8531
	cfg := &nilservice.Config{
		NShards:  5,
		HttpPort: suite.port,
		Topology: collate.TrivialShardTopologyId,
	}
	go nilservice.Run(suite.context, cfg, badger)
	time.Sleep(time.Second) // To be sure that server is started
}

func (suite *SuiteRpc) TearDownTest() {
	suite.cancel()
}

func (s *SuiteRpc) sendRawTransaction(m *types.Message) {
	s.T().Helper()

	resp, err := s.client.SendMessage(m)
	s.Require().NoError(err)
	s.Require().Equal(m.Hash(), resp)
}

func (s *SuiteRpc) getBlockByNumber(shardId types.ShardId, blk string, fullTx bool) *jsonrpc.RPCBlock {
	s.T().Helper()

	var bn transport.BlockNumber
	s.Require().NoError(bn.UnmarshalJSON([]byte(blk)))

	resp, err := s.client.GetBlockByNumber(shardId, bn, fullTx)
	s.Require().NoError(err)
	return resp
}

//nolint:unparam
func (s *SuiteRpc) getTransactionCount(addr types.Address, blk string) types.Seqno {
	s.T().Helper()

	var bn transport.BlockNumber
	s.Require().NoError(bn.UnmarshalJSON([]byte(blk)))

	resp, err := s.client.GetTransactionCount(addr, transport.BlockNumberOrHash{BlockNumber: &bn})
	s.Require().NoError(err)
	return resp
}

func (suite *SuiteRpc) waitForReceiptOnShard(shardId types.ShardId, msg *types.Message) *jsonrpc.RPCReceipt {
	suite.T().Helper()

	var receipt *jsonrpc.RPCReceipt
	var err error
	suite.Require().Eventually(func() bool {
		receipt, err = suite.client.GetInMessageReceipt(shardId, msg.Hash())
		suite.Require().NoError(err)
		return receipt.IsComplete()
	}, 15*time.Second, 200*time.Millisecond)

	suite.Equal(msg.Hash(), receipt.MsgHash)

	return receipt
}

func (suite *SuiteRpc) waitForReceipt(msg *types.Message) *jsonrpc.RPCReceipt {
	suite.T().Helper()

	var shardId types.ShardId
	if msg.Internal {
		shardId = msg.From.ShardId()
	} else {
		shardId = msg.To.ShardId()
	}
	var receipt *jsonrpc.RPCReceipt
	var err error
	suite.Require().Eventually(func() bool {
		receipt, err = suite.client.GetInMessageReceipt(shardId, msg.Hash())
		suite.Require().NoError(err)
		return receipt.IsComplete()
	}, 15*time.Second, 200*time.Millisecond)

	// Check receipt only for the outermost message
	suite.checkReceipt(receipt, msg.Hash())

	return receipt
}

func (suite *SuiteRpc) checkReceipt(receipt *jsonrpc.RPCReceipt, msgHash common.Hash) {
	suite.T().Helper()
	suite.Require().NotNil(receipt)
	suite.Require().True(receipt.Success)
	suite.Require().Equal(msgHash, receipt.MsgHash)
}

func (suite *SuiteRpc) sendDeployMessage(from types.Address, code types.Code, seqno types.Seqno) *jsonrpc.RPCReceipt {
	suite.T().Helper()

	shardId := from.ShardId()

	msg := suite.createMessageForDeploy(from, seqno, code, shardId, 10001)
	suite.Require().NoError(msg.Sign(execution.MainPrivateKey))

	// create contract
	suite.sendRawTransaction(msg)

	// wait for receipt
	return suite.waitForReceiptOnShard(shardId, msg)
}

func (s *SuiteRpc) TestRpcBasic() {
	var someRandomMissingBlock common.Hash
	s.Require().NoError(someRandomMissingBlock.UnmarshalText([]byte("0x6804117de2f3e6ee32953e78ced1db7b20214e0d8c745a03b8fecf7cc8ee76ef")))

	res, err := s.client.GetBlockByNumber(types.BaseShardId, transport.BlockNumber(0x1b4), false)
	s.Require().NoError(err)
	s.Require().Nil(res)

	count, err := s.client.GetBlockTransactionCountByNumber(types.BaseShardId, transport.EarliestBlockNumber)
	s.Require().NoError(err)
	s.EqualValues(0, count)

	count, err = s.client.GetBlockTransactionCountByHash(types.BaseShardId, someRandomMissingBlock)
	s.Require().NoError(err)
	s.EqualValues(0, count)

	res, err = s.client.GetBlockByHash(types.BaseShardId, someRandomMissingBlock, false)
	s.Require().NoError(err)
	s.Require().Nil(res)

	res, err = s.client.GetBlockByNumber(types.BaseShardId, transport.EarliestBlockNumber, false)
	s.Require().NoError(err)
	s.Require().NotNil(res)

	latest, err := s.client.GetBlockByNumber(types.BaseShardId, transport.LatestBlockNumber, false)
	s.Require().NoError(err)
	s.Require().NotNil(res)

	res, err = s.client.GetBlockByHash(types.BaseShardId, latest.Hash, false)
	s.Require().NoError(err)
	s.Require().Equal(latest, res)

	msg, err := s.client.GetInMessageByHash(types.BaseShardId, someRandomMissingBlock)
	s.Require().NoError(err)
	s.Require().Nil(msg)
}

func (suite *SuiteRpc) createMessageForDeploy(
	from types.Address, seqno types.Seqno, code types.Code, toShard types.ShardId, gas uint64,
) *types.Message {
	suite.T().Helper()

	dm := types.BuildDeployPayload(code, common.EmptyHash)

	m := &types.Message{
		Seqno:    seqno,
		Data:     dm.Bytes(),
		From:     from,
		GasLimit: *types.NewUint256(gas),
		To:       types.CreateAddress(toShard, code),
		Internal: true,
		Deploy:   true,
	}
	suite.address = m.To
	return m
}

func (suite *SuiteRpc) loadContract(path string, name string) (types.Code, abi.ABI) {
	suite.T().Helper()

	contracts, err := solc.CompileSource(path)
	suite.Require().NoError(err)
	code := hexutil.FromHex(contracts[name].Code)
	abi := solc.ExtractABI(contracts[name])
	return code, abi
}

func (suite *SuiteRpc) TestRpcContract() {
	contractCode, abi := suite.loadContract(common.GetAbsolutePath("./contracts/increment.sol"), "Incrementer")

	addr := suite.deployContractViaWallet(1, contractCode)

	// now call (= send a message to) created contract
	seqno := suite.getTransactionCount(addr, "latest")

	calldata, err := abi.Pack("increment")
	suite.Require().NoError(err)
	messageToSend := &types.Message{
		Seqno:    seqno,
		From:     types.MainWalletAddress,
		To:       addr,
		Data:     calldata,
		GasLimit: *types.NewUint256(100003),
		Internal: true,
	}
	suite.Require().NoError(messageToSend.Sign(execution.MainPrivateKey))

	suite.sendMessageViaWallet(types.MainWalletAddress, messageToSend)
}

func (suite *SuiteRpc) TestRpcDeployToMainShard() {
	pub := crypto.CompressPubkey(&execution.MainPrivateKey.PublicKey)
	from := types.PubkeyBytesToAddress(types.MasterShardId, pub)

	seqno := suite.getTransactionCount(from, "latest")

	code, _ := suite.loadContract(common.GetAbsolutePath("./contracts/increment.sol"), "Incrementer")
	receipt := suite.sendDeployMessage(from, code, seqno)
	suite.False(receipt.Success)
}

func (suite *SuiteRpc) TestRpcContractSendMessage() {
	// deploy caller contract
	callerCode, callerAbi := suite.loadContract(common.GetAbsolutePath("./contracts/async_call.sol"), "Caller")
	callerAddr := suite.deployContractViaWallet(types.MasterShardId, callerCode)

	checkForShard := func(shardId types.ShardId) {
		suite.T().Helper()

		// deploy callee contracts to different shards
		calleeCode, calleeAbi := suite.loadContract(common.GetAbsolutePath("./contracts/async_call.sol"), "Callee")
		calleeAddr := suite.deployContractViaWallet(shardId, calleeCode)

		// pack call of Callee::add into message
		calldata, err := calleeAbi.Pack("add", int32(123))
		suite.Require().NoError(err)
		messageToSend := &types.Message{
			Seqno:    suite.getTransactionCount(callerAddr, "latest"),
			Data:     calldata,
			From:     callerAddr,
			To:       calleeAddr,
			Value:    *types.NewUint256(0),
			GasLimit: *types.NewUint256(100004),
			Internal: true,
		}
		calldata, err = messageToSend.MarshalSSZ()
		suite.Require().NoError(err)

		// now call Caller::send_message
		callerSeqno := suite.getTransactionCount(callerAddr, "latest")
		calldata, err = callerAbi.Pack("send_msg", calldata)
		suite.Require().NoError(err)

		callCallerMethod := &types.Message{
			Seqno:    callerSeqno,
			From:     callerAddr,
			To:       callerAddr,
			Data:     calldata,
			GasLimit: *types.NewUint256(10005),
			Internal: false,
		}
		suite.Require().NoError(callCallerMethod.Sign(execution.MainPrivateKey))
		suite.sendRawTransaction(callCallerMethod)

		suite.waitForReceipt(callCallerMethod)
	}

	// check that we can call contract from neighbor shard
	checkForShard(types.ShardId(4))

	// check that we can also send message to the same shard
	checkForShard(types.BaseShardId)
}

func (suite *SuiteRpc) TestRpcApiModules() {
	res, err := suite.client.Call("rpc_modules")
	suite.Require().NoError(err)
	suite.Equal("1.0", res["eth"])
	suite.Equal("1.0", res["rpc"])
}

func (suite *SuiteRpc) TestRpcError() {
	check := func(code int, msg, method string, params ...any) {
		resp, err := suite.client.Call(method, params...)
		suite.Require().ErrorContains(err, strconv.Itoa(code))
		suite.Require().ErrorContains(err, msg)
		suite.Require().Nil(resp)
	}

	check(-32601, "the method eth_doesntExist does not exist/is not available",
		"eth_doesntExist")

	check(-32602, "missing value for required argument 0",
		rpc_client.Eth_getBlockByNumber)

	check(-32602, "invalid argument 0: json: cannot unmarshal number 1099511627776 into Go value of type uint32",
		rpc_client.Eth_getBlockByNumber, 1<<40)

	check(-32602, "missing value for required argument 1",
		rpc_client.Eth_getBlockByNumber, types.BaseShardId)

	check(-32602, "invalid argument 1: hex string of odd length",
		rpc_client.Eth_getBlockByHash, types.BaseShardId, "0x1b4", false)

	check(-32602, "invalid argument 1: hex string without 0x prefix",
		rpc_client.Eth_getBlockByHash, types.BaseShardId, "latest")
}

func (suite *SuiteRpc) TestRpcDebugModules() {
	res, err := suite.client.Call("debug_getBlockByNumber", types.BaseShardId, "latest", false)
	suite.Require().NoError(err)

	suite.Require().Contains(res, "number")
	suite.Require().Contains(res, "hash")
	suite.Require().Contains(res, "content")

	sliceContent, ok := res["content"].(string)
	suite.Require().True(ok)
	// check if the string starts with 0x prefix
	suite.Require().Equal("0x", sliceContent[:2])
	// print resp to see the result
	suite.T().Logf("resp: %v", res)

	fullRes, err := suite.client.Call("debug_getBlockByNumber", types.BaseShardId, "latest", true)
	suite.Require().NoError(err)
	suite.Require().Contains(fullRes, "content")
	suite.Require().Contains(fullRes, "inMessages")
	suite.Require().Contains(fullRes, "outMessages")
	suite.Require().Contains(fullRes, "receipts")
}

func TestSuiteRpc(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteRpc))
}
