package rpctest

import (
	"testing"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/NilFoundation/nil/nil/internal/abi"
	"github.com/NilFoundation/nil/nil/internal/contracts"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/nilservice"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
	"github.com/stretchr/testify/suite"
)

type SuiteAsyncAwait struct {
	RpcSuite
	testAddress0    types.Address
	testAddress1    types.Address
	counterAddress0 types.Address
	counterAddress1 types.Address
	abiTest         *abi.ABI
	abiCounter      *abi.ABI
	zerostateCfg    string
	accounts        []types.Address
}

func (s *SuiteAsyncAwait) SetupSuite() {
	s.shardsNum = 4

	var err error
	s.testAddress0, err = contracts.CalculateAddress(contracts.NameAwaitTest, 1, []byte{1})
	s.Require().NoError(err)
	s.testAddress1, err = contracts.CalculateAddress(contracts.NameAwaitTest, 2, []byte{2})
	s.Require().NoError(err)
	s.counterAddress0, err = contracts.CalculateAddress(contracts.NameCounter, 1, []byte{1})
	s.Require().NoError(err)
	s.counterAddress1, err = contracts.CalculateAddress(contracts.NameCounter, 2, []byte{2})
	s.Require().NoError(err)

	s.accounts = append(s.accounts, types.MainWalletAddress)
	s.accounts = append(s.accounts, s.testAddress0)
	s.accounts = append(s.accounts, s.testAddress1)
	s.accounts = append(s.accounts, s.counterAddress0)
	s.accounts = append(s.accounts, s.counterAddress1)

	s.abiTest, err = contracts.GetAbi(contracts.NameAwaitTest)
	s.Require().NoError(err)

	s.abiCounter, err = contracts.GetAbi(contracts.NameCounter)
	s.Require().NoError(err)

	zerostateTmpl := `
contracts:
- name: MainWallet
  address: {{ .MainWalletAddress }}
  value: 100000000000000
  contract: Wallet
  ctorArgs: [{{ .MainPublicKey }}]
- name: Test0
  address: {{ .TestAddress0 }}
  value: 100000000000000
  contract: tests/AwaitTest
- name: Test1
  address: {{ .TestAddress1 }}
  value: 100000000000000
  contract: tests/AwaitTest
- name: Counter0
  address: {{ .CounterAddress0 }}
  value: 100000000000000
  contract: tests/Counter
- name: Counter1
  address: {{ .CounterAddress1 }}
  value: 100000000000000
  contract: tests/Counter
`
	s.zerostateCfg, err = common.ParseTemplate(zerostateTmpl, map[string]interface{}{
		"MainPublicKey":     hexutil.Encode(execution.MainPublicKey),
		"MainWalletAddress": types.MainWalletAddress.Hex(),
		"TestAddress0":      s.testAddress0.Hex(),
		"TestAddress1":      s.testAddress1.Hex(),
		"CounterAddress0":   s.counterAddress0.Hex(),
		"CounterAddress1":   s.counterAddress1.Hex(),
	})
	s.Require().NoError(err)
}

func (s *SuiteAsyncAwait) SetupTest() {
	s.start(&nilservice.Config{
		NShards:       s.shardsNum,
		ZeroStateYaml: s.zerostateCfg,
		RunMode:       nilservice.CollatorsOnlyRunMode,
	})
}

func (s *SuiteAsyncAwait) TearDownTest() {
	s.cancel()
}

func (s *SuiteAsyncAwait) UpdateBalance() types.Value {
	s.T().Helper()

	balance := types.NewZeroValue()
	for _, addr := range s.accounts {
		balance = balance.Add(s.getBalance(addr))
	}
	return balance
}

func (s *SuiteAsyncAwait) TestSumCounters() {
	var (
		data    []byte
		receipt *jsonrpc.RPCReceipt
	)

	data = s.AbiPack(s.abiCounter, "add", int32(11))
	receipt = s.sendExternalMessageNoCheck(data, s.counterAddress0)
	s.Require().True(receipt.AllSuccess())

	data = s.AbiPack(s.abiCounter, "add", int32(456))
	receipt = s.sendExternalMessageNoCheck(data, s.counterAddress1)
	s.Require().True(receipt.AllSuccess())

	initialBalance := s.UpdateBalance()

	data = s.AbiPack(s.abiTest, "sumCounters", []types.Address{s.counterAddress0, s.counterAddress1, s.testAddress0})
	receipt = s.sendExternalMessageNoCheck(data, s.testAddress0)
	s.Require().True(receipt.AllSuccess())

	info := s.analyzeReceipt(receipt, map[types.Address]string{})
	s.checkBalance(info, initialBalance, s.accounts)

	data = s.AbiPack(s.abiTest, "value")
	data = s.CallGetter(s.testAddress0, data, "latest", nil)
	nameRes, err := s.abiTest.Unpack("value", data)
	s.Require().NoError(err)
	value, ok := nameRes[0].(int32)
	s.Require().True(ok)
	s.Require().Equal(int32(467*2), value)
}

func (s *SuiteAsyncAwait) TestFailed() {
	var (
		data            []byte
		receipt         *jsonrpc.RPCReceipt
		responseReceipt *jsonrpc.RPCReceipt
		info            ReceiptInfo
	)

	initialBalance := s.UpdateBalance()

	s.Run("callFailed with false fail flag", func() {
		data = s.AbiPack(s.abiTest, "callFailed", s.testAddress1, false)
		receipt = s.sendExternalMessageNoCheck(data, s.testAddress0)
		s.Require().True(receipt.AllSuccess())

		info = s.analyzeReceipt(receipt, map[types.Address]string{})
		initialBalance = s.checkBalance(info, initialBalance, s.accounts)

		responseReceipt = receipt.OutReceipts[0].OutReceipts[0]
		s.Require().Len(responseReceipt.Logs, 1)
		s.Require().Equal(s.abiTest.Events["awaitCallResult"].ID.Bytes(), responseReceipt.Logs[0].Topics[0].Bytes())
		args, err := s.abiTest.Events["awaitCallResult"].Inputs.Unpack(responseReceipt.Logs[0].Data)
		s.Require().NoError(err)
		success, ok := args[0].(bool)
		s.Require().True(ok)
		s.Require().True(success)
	})

	s.Run("callFailed with true fail flag", func() {
		data = s.AbiPack(s.abiTest, "callFailed", s.testAddress1, true)
		receipt = s.sendExternalMessageNoCheck(data, s.testAddress0)
		s.Require().True(receipt.Success)
		// `checkFail` method should revert
		s.Require().False(receipt.OutReceipts[0].Success)

		responseReceipt = receipt.OutReceipts[0].OutReceipts[0]
		s.Require().True(responseReceipt.Success)
		s.Require().Len(responseReceipt.Logs, 1)
		args, err := s.abiTest.Events["awaitCallResult"].Inputs.Unpack(responseReceipt.Logs[0].Data)
		s.Require().NoError(err)
		success, ok := args[0].(bool)
		s.Require().True(ok)
		s.Require().False(success)

		info = s.analyzeReceipt(receipt, map[types.Address]string{})
		s.checkBalance(info, initialBalance, s.accounts)
	})
}

func (s *SuiteAsyncAwait) TestFactorial() {
	data := s.AbiPack(s.abiTest, "factorial", int32(10))
	receipt := s.sendExternalMessageNoCheck(data, s.testAddress0)
	s.Require().True(receipt.AllSuccess())

	data = s.AbiPack(s.abiTest, "value")
	data = s.CallGetter(s.testAddress0, data, "latest", nil)
	nameRes, err := s.abiTest.Unpack("value", data)
	s.Require().NoError(err)
	value, ok := nameRes[0].(int32)
	s.Require().True(ok)
	s.Require().Equal(int32(3628800), value)
}

func (s *SuiteAsyncAwait) TestSumCountersNested() {
	var (
		data    []byte
		receipt *jsonrpc.RPCReceipt
	)

	data = s.AbiPack(s.abiCounter, "add", int32(11))
	receipt = s.sendExternalMessageNoCheck(data, s.counterAddress0)
	s.Require().True(receipt.AllSuccess())

	data = s.AbiPack(s.abiCounter, "add", int32(22))
	receipt = s.sendExternalMessageNoCheck(data, s.counterAddress1)
	s.Require().True(receipt.AllSuccess())

	data = s.AbiPack(s.abiTest, "sumCountersNested", []types.Address{s.testAddress0, s.testAddress1},
		[]types.Address{s.counterAddress0, s.counterAddress1})
	receipt = s.sendExternalMessageNoCheck(data, s.testAddress0)
	s.Require().True(receipt.AllSuccess())

	data = s.AbiPack(s.abiTest, "value")
	data = s.CallGetter(s.testAddress0, data, "latest", nil)
	nameRes, err := s.abiTest.Unpack("value", data)
	s.Require().NoError(err)
	s.Require().NotEmpty(nameRes)
	value, ok := nameRes[0].(int32)
	s.Require().True(ok)
	s.Require().Equal(int32(33), value)
}

func (s *SuiteAsyncAwait) TestTestNoneZeroCallDepth() {
	data := s.AbiPack(s.abiTest, "testNoneZeroCallDepth", s.testAddress0)
	receipt := s.sendExternalMessageNoCheck(data, s.testAddress0)
	s.Require().False(receipt.AllSuccess())
	s.Require().Equal("PrecompileReverted", receipt.Status)
}

func TestAsyncAwait(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteAsyncAwait))
}
