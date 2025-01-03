package main

import (
	"math/big"
	"testing"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/NilFoundation/nil/nil/internal/abi"
	"github.com/NilFoundation/nil/nil/internal/contracts"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/network"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/nilservice"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
	"github.com/NilFoundation/nil/nil/tests"
	"github.com/stretchr/testify/suite"
)

type SuiteAsyncAwait struct {
	tests.ShardedSuite

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
	var err error
	s.testAddress0, err = contracts.CalculateAddress(contracts.NameRequestResponseTest, 1, []byte{1})
	s.Require().NoError(err)
	s.testAddress1, err = contracts.CalculateAddress(contracts.NameRequestResponseTest, 2, []byte{2})
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

	s.abiTest, err = contracts.GetAbi(contracts.NameRequestResponseTest)
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
  contract: tests/RequestResponseTest
- name: Test1
  address: {{ .TestAddress1 }}
  value: 0
  contract: tests/RequestResponseTest
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
	nShards := 4
	port := 10425
	s.Start(&nilservice.Config{
		NShards:       uint32(nShards),
		ZeroStateYaml: s.zerostateCfg,
	}, port)

	_, archiveNodeAddr := s.StartArchiveNode(port+nShards, true)
	s.DefaultClient, _ = s.StartRPCNode(tests.WithDhtBootstrapByValidators, network.AddrInfoSlice{archiveNodeAddr})
}

func (s *SuiteAsyncAwait) TearDownTest() {
	s.Cancel()
}

func (s *SuiteAsyncAwait) UpdateBalance() types.Value {
	s.T().Helper()

	balance := types.NewZeroValue()
	for _, addr := range s.accounts {
		balance = balance.Add(s.GetBalance(addr))
	}
	return balance
}

func (s *SuiteAsyncAwait) TestSumCounters() {
	var (
		data    []byte
		receipt *jsonrpc.RPCReceipt
	)

	data = s.AbiPack(s.abiCounter, "add", int32(11))
	receipt = s.SendExternalMessageNoCheck(data, s.counterAddress0)
	s.Require().True(receipt.AllSuccess())

	data = s.AbiPack(s.abiCounter, "add", int32(456))
	receipt = s.SendExternalMessageNoCheck(data, s.counterAddress1)
	s.Require().True(receipt.AllSuccess())

	initialBalance := s.UpdateBalance()

	data = s.AbiPack(s.abiTest, "sumCounters", []types.Address{s.counterAddress0, s.counterAddress1, s.testAddress0})
	receipt = s.SendExternalMessageNoCheck(data, s.testAddress0)
	s.Require().True(receipt.AllSuccess())

	info := s.AnalyzeReceipt(receipt, map[types.Address]string{})
	// we use `receipt.GasUsed` field in calculations here
	// this gives a slightly different result since part of the "spent" gas
	// in fact is being reserved for the response processing and later is being refunded
	// TODO: likely we need to introduce `receipt.GasReserved` field as well

	// 3 async calls; 60_000 is value from the RequestResponseTest.sol
	valueReservedAsync := types.Gas(3 * 60_000).ToValue(types.DefaultGasPrice)
	s.CheckBalance(info, initialBalance.Add(valueReservedAsync), s.accounts)

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
		info            tests.ReceiptInfo
	)

	initialBalance := s.UpdateBalance()
	// we use `receipt.GasUsed` field in calculations here
	// this gives a slightly different result since part of the "spent" gas
	// in fact is being reserved for the response processing and later is being refunded
	// TODO: likely we need to introduce `receipt.GasReserved` field as well

	valueReservedAsync := types.Gas(50_000).ToValue(types.DefaultGasPrice)

	s.Run("callFailed with false fail flag", func() {
		data = s.AbiPack(s.abiTest, "callFailed", s.testAddress1, false)
		receipt = s.SendExternalMessageNoCheck(data, s.testAddress0)
		s.Require().True(receipt.AllSuccess())

		info = s.AnalyzeReceipt(receipt, map[types.Address]string{})
		initialBalance = s.CheckBalance(info, initialBalance.Add(valueReservedAsync), s.accounts)

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
		receipt = s.SendExternalMessageNoCheck(data, s.testAddress0)
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

		info = s.AnalyzeReceipt(receipt, map[types.Address]string{})
		s.CheckBalance(info, initialBalance.Add(valueReservedAsync), s.accounts)
	})
}

func (s *SuiteAsyncAwait) TestFactorial() {
	data := s.AbiPack(s.abiTest, "factorial", int32(6))
	receipt := s.SendExternalMessageNoCheck(data, s.testAddress0)
	s.Require().True(receipt.AllSuccess())

	data = s.AbiPack(s.abiTest, "value")
	data = s.CallGetter(s.testAddress0, data, "latest", nil)
	nameRes, err := s.abiTest.Unpack("value", data)
	s.Require().NoError(err)
	value, ok := nameRes[0].(int32)
	s.Require().True(ok)
	s.Require().Equal(int32(720), value)
}

func (s *SuiteAsyncAwait) TestFibonacci() {
	data := s.AbiPack(s.abiTest, "fibonacci", int32(6))
	receipt := s.SendExternalMessageNoCheck(data, s.testAddress0)
	s.Require().True(receipt.AllSuccess())

	data = s.AbiPack(s.abiTest, "value")
	data = s.CallGetter(s.testAddress0, data, "latest", nil)
	nameRes, err := s.abiTest.Unpack("value", data)
	s.Require().NoError(err)
	value, ok := nameRes[0].(int32)
	s.Require().True(ok)
	s.Require().Equal(int32(8), value)
}

func (s *SuiteAsyncAwait) TestTwoRequests() {
	var (
		data    []byte
		receipt *jsonrpc.RPCReceipt
	)

	data = s.AbiPack(s.abiCounter, "add", int32(11))
	receipt = s.SendExternalMessageNoCheck(data, s.counterAddress0)
	s.Require().True(receipt.AllSuccess())

	data = s.AbiPack(s.abiCounter, "add", int32(456))
	receipt = s.SendExternalMessageNoCheck(data, s.counterAddress1)
	s.Require().True(receipt.AllSuccess())

	data = s.AbiPack(s.abiTest, "makeTwoRequests", s.counterAddress0, s.counterAddress1)
	receipt = s.SendExternalMessageNoCheck(data, s.testAddress0)
	s.Require().True(receipt.AllSuccess())

	data = s.AbiPack(s.abiTest, "value")
	data = s.CallGetter(s.testAddress0, data, "latest", nil)
	nameRes, err := s.abiTest.Unpack("value", data)
	s.Require().NoError(err)
	value, ok := nameRes[0].(int32)
	s.Require().True(ok)
	s.Require().EqualValues(11+456, value)
}

func (s *SuiteAsyncAwait) TestInvalidContext() {
	data := s.AbiPack(s.abiTest, "makeInvalidContext", s.counterAddress0)
	receipt := s.SendExternalMessageNoCheck(data, s.testAddress0)
	s.Require().True(receipt.Success)
	s.Require().False(receipt.OutReceipts[0].Success)
}

func (s *SuiteAsyncAwait) TestInvalidSendRequest() {
	data := s.AbiPack(s.abiTest, "makeInvalidSendRequest")
	receipt := s.SendExternalMessageNoCheck(data, s.testAddress0)
	s.Require().True(receipt.Success)
	s.Empty(receipt.OutReceipts)
}

func (s *SuiteAsyncAwait) TestSumCountersNested() {
	var (
		data    []byte
		receipt *jsonrpc.RPCReceipt
	)

	data = s.AbiPack(s.abiCounter, "add", int32(11))
	receipt = s.SendExternalMessageNoCheck(data, s.counterAddress0)
	s.Require().True(receipt.AllSuccess())

	data = s.AbiPack(s.abiCounter, "add", int32(22))
	receipt = s.SendExternalMessageNoCheck(data, s.counterAddress1)
	s.Require().True(receipt.AllSuccess())

	data = s.AbiPack(s.abiTest, "sumCountersNested", []types.Address{s.testAddress0, s.testAddress1},
		[]types.Address{s.counterAddress0, s.counterAddress1})
	receipt = s.SendExternalMessageNoCheck(data, s.testAddress0)
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

func (s *SuiteAsyncAwait) TestNoneZeroCallDepth() {
	data := s.AbiPack(s.abiTest, "testNoneZeroCallDepth", s.testAddress0)
	receipt := s.SendExternalMessageNoCheck(data, s.testAddress0)
	s.Require().False(receipt.AllSuccess())
	s.Require().Equal("AwaitCallCalledFromNotTopLevel", receipt.Status)
}

func (s *SuiteAsyncAwait) TestRequestResponse() {
	var info tests.ReceiptInfo

	s.Run("Add to counter", func() {
		data := s.AbiPack(s.abiCounter, "add", int32(123))
		receipt := s.SendExternalMessageNoCheck(data, s.counterAddress0)
		s.Require().True(receipt.AllSuccess())
	})

	initialBalance := s.UpdateBalance()
	// we use `receipt.GasUsed` field in calculations here
	// this gives a slightly different result since part of the "spent" gas
	// in fact is being reserved for the response processing and later is being refunded
	// TODO: likely we need to introduce `receipt.GasReserved` field as well
	valueReservedAsync := types.Gas(50_000).ToValue(types.DefaultGasPrice)

	s.Run("Call Counter.get", func() {
		intContext := big.NewInt(456)
		strContext := "Hello World"

		data := s.AbiPack(s.abiTest, "requestCounterGet", s.counterAddress0, intContext, strContext)
		receipt := s.SendExternalMessageNoCheck(data, s.testAddress0)
		s.Require().True(receipt.AllSuccess())

		tests.CheckContractValueEqual(s.T(), s.Context, s.DefaultClient, s.abiTest, s.testAddress0, "counterValue", int32(123))
		tests.CheckContractValueEqual(s.T(), s.Context, s.DefaultClient, s.abiTest, s.testAddress0, "intValue", intContext)
		tests.CheckContractValueEqual(s.T(), s.Context, s.DefaultClient, s.abiTest, s.testAddress0, "strValue", "Hello World")

		info = s.AnalyzeReceipt(receipt, map[types.Address]string{})

		initialBalance = s.CheckBalance(info, initialBalance.Add(valueReservedAsync), s.accounts)
		s.checkAsyncContextEmpty(s.testAddress0)
	})

	s.Run("Call Counter.add", func() {
		data := s.AbiPack(s.abiTest, "requestCounterAdd", s.counterAddress0, int32(100))
		receipt := s.SendExternalMessageNoCheck(data, s.testAddress0)
		s.Require().True(receipt.AllSuccess())

		tests.CheckContractValueEqual(s.T(), s.Context, s.DefaultClient, s.abiCounter, s.counterAddress0, "get", int32(223))

		info = s.AnalyzeReceipt(receipt, map[types.Address]string{})
		initialBalance = s.CheckBalance(info, initialBalance.Add(valueReservedAsync), s.accounts)
		s.checkAsyncContextEmpty(s.testAddress0)
	})

	s.Run("Test failed request with value", func() {
		data := s.AbiPack(s.abiTest, "requestCheckFail", s.testAddress1, true)
		receipt := s.SendExternalMessageNoCheck(data, s.testAddress0)
		s.Require().False(receipt.AllSuccess())
		s.Require().Len(receipt.OutReceipts, 1)
		requestReceipt := receipt.OutReceipts[0]
		s.Require().Len(requestReceipt.OutReceipts, 1)
		responseReceipt := requestReceipt.OutReceipts[0]

		s.Require().False(requestReceipt.Success)
		s.Require().Equal("ExecutionReverted", requestReceipt.Status)
		s.Require().True(responseReceipt.Success)

		info = s.AnalyzeReceipt(receipt, map[types.Address]string{})
		initialBalance = s.CheckBalance(info, initialBalance.Add(valueReservedAsync), s.accounts)
		s.checkAsyncContextEmpty(s.testAddress0)
	})

	s.Run("In case of fail, context trie should not be changed", func() {
		data := s.AbiPack(s.abiTest, "failDuringRequestSending", s.counterAddress0)
		receipt := s.SendExternalMessageNoCheck(data, s.testAddress0)
		s.Require().False(receipt.AllSuccess())

		info = s.AnalyzeReceipt(receipt, map[types.Address]string{})
		initialBalance = s.CheckBalance(info, initialBalance, s.accounts)
		s.checkAsyncContextEmpty(s.testAddress0)
	})

	s.Run("Send currency", func() {
		data := s.AbiPack(s.abiTest, "mintCurrency", big.NewInt(1_000_000))
		receipt := s.SendExternalMessageNoCheck(data, s.testAddress0)
		s.Require().True(receipt.AllSuccess())

		info = s.AnalyzeReceipt(receipt, map[types.Address]string{})
		initialBalance = s.CheckBalance(info, initialBalance, s.accounts)
		s.checkAsyncContextEmpty(s.testAddress0)

		data = s.AbiPack(s.abiTest, "requestSendCurrency", s.counterAddress0, big.NewInt(400_000))
		receipt = s.SendExternalMessageNoCheck(data, s.testAddress0)
		s.Require().True(receipt.AllSuccess())

		info = s.AnalyzeReceipt(receipt, map[types.Address]string{})
		initialBalance = s.CheckBalance(info, initialBalance.Add(valueReservedAsync), s.accounts)
		s.checkAsyncContextEmpty(s.testAddress0)

		currencyId := types.CurrencyId(s.testAddress0)

		currencies, err := s.DefaultClient.GetCurrencies(s.Context, s.testAddress0, "latest")
		s.Require().NoError(err)
		s.Require().Len(currencies, 1)
		s.Equal(types.NewValueFromUint64(600_000), currencies[currencyId])

		currencies, err = s.DefaultClient.GetCurrencies(s.Context, s.counterAddress0, "latest")
		s.Require().NoError(err)
		s.Require().Len(currencies, 1)
		s.Equal(types.NewValueFromUint64(400_000), currencies[currencyId])
	})

	// TODO: support
	// s.Run("Out of gas response", func() {
	//	data := s.AbiPack(s.abiTest, "requestOutOfGasFailure", s.testAddress1)
	//	receipt := s.SendExternalMessageNoCheck(data, s.testAddress0)
	//	s.Require().False(receipt.AllSuccess())
	//	s.Require().Len(receipt.OutReceipts, 1)
	//	requestReceipt := receipt.OutReceipts[0]
	//	s.Require().Len(requestReceipt.OutReceipts, 1)
	//	responseReceipt := requestReceipt.OutReceipts[0]
	//
	//	s.Require().False(requestReceipt.Success)
	//	s.Require().Equal("OutOfGas", requestReceipt.Status)
	//	s.Require().True(responseReceipt.Success)
	//	info = s.AnalyzeReceipt(receipt, map[types.Address]string{})
	//	initialBalance = s.CheckBalance(info, initialBalance, s.accounts)
	//	s.checkAsyncContextEmpty(s.testAddress0)
	// })
}

func (s *SuiteAsyncAwait) TestOnlyResponse() {
	data := s.AbiPack(s.abiTest, "responseCounterAdd", true, []byte{}, []byte{})
	receipt := s.SendExternalMessageNoCheck(data, s.testAddress0)
	s.Require().False(receipt.Success)
	s.Require().Equal("OnlyResponseCheckFailed", receipt.Status)
}

func (s *SuiteAsyncAwait) checkAsyncContextEmpty(address types.Address) {
	s.T().Helper()

	contract := tests.GetContract(s.T(), s.Context, s.Shards[address.ShardId()].Db, address)
	s.Require().Equal(common.EmptyHash, contract.AsyncContextRoot)
}

func TestAsyncAwait(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteAsyncAwait))
}
