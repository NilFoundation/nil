package main

import (
	"math/big"
	"testing"
	"time"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/abi"
	"github.com/NilFoundation/nil/nil/internal/contracts"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/nilservice"
	"github.com/NilFoundation/nil/nil/services/rpc"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
	"github.com/NilFoundation/nil/nil/tests"
	"github.com/stretchr/testify/suite"
)

type SuiteRequestResponse struct {
	tests.ShardedSuite

	testAddress0    types.Address
	testAddress1    types.Address
	counterAddress0 types.Address
	counterAddress1 types.Address
	abiTest         *abi.ABI
	abiCounter      *abi.ABI
	accounts        []types.Address
}

func (s *SuiteRequestResponse) SetupSuite() {
	var err error
	s.testAddress0, err = contracts.CalculateAddress(contracts.NameRequestResponseTest, 1, []byte{1})
	s.Require().NoError(err)
	s.testAddress1, err = contracts.CalculateAddress(contracts.NameRequestResponseTest, 2, []byte{2})
	s.Require().NoError(err)
	s.counterAddress0, err = contracts.CalculateAddress(contracts.NameCounter, 1, []byte{1})
	s.Require().NoError(err)
	s.counterAddress1, err = contracts.CalculateAddress(contracts.NameCounter, 2, []byte{2})
	s.Require().NoError(err)

	s.accounts = append(s.accounts, types.MainSmartAccountAddress)
	s.accounts = append(s.accounts, s.testAddress0)
	s.accounts = append(s.accounts, s.testAddress1)
	s.accounts = append(s.accounts, s.counterAddress0)
	s.accounts = append(s.accounts, s.counterAddress1)

	s.abiTest, err = contracts.GetAbi(contracts.NameRequestResponseTest)
	s.Require().NoError(err)

	s.abiCounter, err = contracts.GetAbi(contracts.NameCounter)
	s.Require().NoError(err)

	nShards := uint32(3)
	port := 10425

	smartAccountValue, err := types.NewValueFromDecimal("100000000000000")
	s.Require().NoError(err)
	zeroState := &execution.ZeroStateConfig{
		Contracts: []*execution.ContractDescr{
			{
				Name:     "MainSmartAccount",
				Contract: "SmartAccount",
				Address:  types.MainSmartAccountAddress,
				Value:    smartAccountValue,
				CtorArgs: []any{execution.MainPublicKey},
			},
			{Name: "Test0", Contract: "tests/RequestResponseTest", Address: s.testAddress0, Value: smartAccountValue},
			{Name: "Test1", Contract: "tests/RequestResponseTest", Address: s.testAddress1, Value: types.Value0},
			{Name: "Counter0", Contract: "tests/Counter", Address: s.counterAddress0, Value: smartAccountValue},
			{Name: "Counter1", Contract: "tests/Counter", Address: s.counterAddress1, Value: smartAccountValue},
		},
	}

	const disableConsensus = true
	s.Start(&nilservice.Config{
		SplitShards:      false,
		HttpUrl:          rpc.GetSockPath(s.T()),
		NShards:          nShards,
		ZeroState:        zeroState,
		DisableConsensus: disableConsensus,
	}, port)

	s.DefaultClient, _ = s.StartRPCNode(&tests.RpcNodeConfig{
		WithDhtBootstrapByValidators: true,
	})
}

func (s *SuiteRequestResponse) SetupTest() {
	data := s.AbiPack(s.abiCounter, "set", int32(0))
	receipt := s.SendExternalTransactionNoCheck(data, s.counterAddress0)
	s.Require().True(receipt.AllSuccess())
	receipt = s.SendExternalTransactionNoCheck(data, s.counterAddress1)
	s.Require().True(receipt.AllSuccess())

	data = s.AbiPack(s.abiTest, "reset")
	receipt = s.SendExternalTransactionNoCheck(data, s.testAddress0)
	s.Require().True(receipt.AllSuccess())
}

func (s *SuiteRequestResponse) TearDownSuite() {
	s.Cancel()
}

func (s *SuiteRequestResponse) UpdateBalance() types.Value {
	s.T().Helper()

	balance := types.NewZeroValue()
	for _, addr := range s.accounts {
		balance = balance.Add(s.GetBalance(addr))
	}
	return balance
}

func (s *SuiteRequestResponse) TestNestedRequest() {
	var (
		data    []byte
		receipt *jsonrpc.RPCReceipt
	)

	data = s.AbiPack(s.abiTest, "nestedRequest", s.testAddress1, s.counterAddress0)
	receipt = s.SendExternalTransactionNoCheck(data, s.testAddress0)
	s.Require().True(receipt.AllSuccess())
}

func (s *SuiteRequestResponse) TestSendRequestFromCallback() {
	var (
		data    []byte
		receipt *jsonrpc.RPCReceipt
	)

	counterValue := tests.CallGetterT[int32](s.T(), s.DefaultClient, s.abiCounter, s.counterAddress0, "get")

	data = s.AbiPack(s.abiTest, "sendRequestFromCallback", s.counterAddress0)
	receipt = s.SendExternalTransactionNoCheck(data, s.testAddress0)
	s.Require().True(receipt.AllSuccess())

	tests.CheckContractValueEqual(s.T(), s.DefaultClient, s.abiCounter, s.counterAddress0, "get",
		counterValue+1+2+3+4+5)

	s.Require().False(receipt.OutReceipts[0].Flags.IsResponse())

	response := receipt.OutReceipts[0].OutReceipts[0]
	s.Require().True(response.Flags.IsResponse())
	s.Require().False(response.OutReceipts[0].Flags.IsResponse())

	response = response.OutReceipts[0].OutReceipts[0]
	s.Require().True(response.Flags.IsResponse())
	s.Require().False(response.OutReceipts[0].Flags.IsResponse())

	response = response.OutReceipts[0].OutReceipts[0]
	s.Require().True(response.Flags.IsResponse())
	s.Require().False(response.OutReceipts[0].Flags.IsResponse())

	response = response.OutReceipts[0].OutReceipts[0]
	s.Require().True(response.Flags.IsResponse())
	s.Require().False(response.OutReceipts[0].Flags.IsResponse())

	response = response.OutReceipts[0].OutReceipts[0]
	s.Require().True(response.Flags.IsResponse())
	s.Require().False(response.OutReceipts[0].Flags.IsResponse())
}

func (s *SuiteRequestResponse) TestTwoRequests() {
	var (
		data    []byte
		receipt *jsonrpc.RPCReceipt
	)

	data = s.AbiPack(s.abiCounter, "add", int32(11))
	receipt = s.SendExternalTransactionNoCheck(data, s.counterAddress0)
	s.Require().True(receipt.AllSuccess())

	data = s.AbiPack(s.abiCounter, "add", int32(456))
	receipt = s.SendExternalTransactionNoCheck(data, s.counterAddress1)
	s.Require().True(receipt.AllSuccess())

	data = s.AbiPack(s.abiTest, "makeTwoRequests", s.counterAddress0, s.counterAddress1)
	hash, err := s.DefaultClient.SendExternalTransaction(s.T().Context(), data, s.testAddress0, nil, types.FeePack{})
	s.Require().NoError(err)

	s.Eventually(func() bool {
		debugContract, err := s.DefaultClient.GetDebugContract(s.Context, s.testAddress0, "latest")
		s.Require().NoError(err)
		return len(debugContract.AsyncContext) > 0
	}, tests.BlockWaitTimeout, time.Duration(s.Instances[0].Config.CollatorTickPeriodMs/5)*time.Millisecond)

	receipt = s.WaitIncludedInMain(hash)
	s.Require().True(receipt.AllSuccess())

	data = s.AbiPack(s.abiTest, "value")
	data = s.CallGetter(s.testAddress0, data, "latest", nil)
	nameRes, err := s.abiTest.Unpack("value", data)
	s.Require().NoError(err)
	value, ok := nameRes[0].(int32)
	s.Require().True(ok)
	s.Require().EqualValues(11+456, value)
}

func (s *SuiteRequestResponse) TestInvalidContext() {
	data := s.AbiPack(s.abiTest, "makeInvalidContext", s.counterAddress0)
	receipt := s.SendExternalTransactionNoCheck(data, s.testAddress0)
	s.Require().True(receipt.Success)
	s.Require().False(receipt.OutReceipts[0].Success)
}

func (s *SuiteRequestResponse) TestInvalidSendRequest() {
	data := s.AbiPack(s.abiTest, "makeInvalidSendRequest")
	receipt := s.SendExternalTransactionNoCheck(data, s.testAddress0)
	s.Require().True(receipt.Success)
	s.Empty(receipt.OutReceipts)
}

func (s *SuiteRequestResponse) TestRequestResponse() {
	var info tests.ReceiptInfo

	s.Run("Add to counter", func() {
		data := s.AbiPack(s.abiCounter, "add", int32(123))
		receipt := s.SendExternalTransactionNoCheck(data, s.counterAddress0)
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
		receipt := s.SendExternalTransactionNoCheck(data, s.testAddress0)
		s.Require().True(receipt.AllSuccess())

		tests.CheckContractValueEqual(
			s.T(), s.DefaultClient, s.abiTest, s.testAddress0, "counterValue", int32(123))
		tests.CheckContractValueEqual(
			s.T(), s.DefaultClient, s.abiTest, s.testAddress0, "intValue", intContext)
		tests.CheckContractValueEqual(
			s.T(), s.DefaultClient, s.abiTest, s.testAddress0, "strValue", "Hello World")

		info = s.AnalyzeReceipt(receipt, map[types.Address]string{})

		initialBalance = s.CheckBalance(info, initialBalance.Add(valueReservedAsync), s.accounts)
		s.checkAsyncContextEmpty(s.testAddress0)
	})

	s.Run("Call Counter.add", func() {
		data := s.AbiPack(s.abiTest, "requestCounterAdd", s.counterAddress0, int32(100))
		receipt := s.SendExternalTransactionNoCheck(data, s.testAddress0)
		s.Require().True(receipt.AllSuccess())

		tests.CheckContractValueEqual(
			s.T(), s.DefaultClient, s.abiCounter, s.counterAddress0, "get", int32(223))

		info = s.AnalyzeReceipt(receipt, map[types.Address]string{})
		initialBalance = s.CheckBalance(info, initialBalance.Add(valueReservedAsync), s.accounts)
		s.checkAsyncContextEmpty(s.testAddress0)
	})

	s.Run("Test failed request with value", func() {
		data := s.AbiPack(s.abiTest, "requestCheckFail", s.testAddress1, true)
		receipt := s.SendExternalTransactionNoCheck(data, s.testAddress0)
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
		receipt := s.SendExternalTransactionNoCheck(data, s.testAddress0)
		s.Require().False(receipt.AllSuccess())

		info = s.AnalyzeReceipt(receipt, map[types.Address]string{})
		initialBalance = s.CheckBalance(info, initialBalance, s.accounts)
		s.checkAsyncContextEmpty(s.testAddress0)
	})

	s.Run("Send token", func() {
		data := s.AbiPack(s.abiTest, "mintToken", big.NewInt(1_000_000))
		receipt := s.SendExternalTransactionNoCheck(data, s.testAddress0)
		s.Require().True(receipt.AllSuccess())

		info = s.AnalyzeReceipt(receipt, map[types.Address]string{})
		initialBalance = s.CheckBalance(info, initialBalance, s.accounts)
		s.checkAsyncContextEmpty(s.testAddress0)

		data = s.AbiPack(s.abiTest, "requestSendToken", s.counterAddress0, big.NewInt(400_000))
		receipt = s.SendExternalTransactionNoCheck(data, s.testAddress0)
		s.Require().True(receipt.AllSuccess())

		info = s.AnalyzeReceipt(receipt, map[types.Address]string{})
		initialBalance = s.CheckBalance(info, initialBalance.Add(valueReservedAsync), s.accounts)
		s.checkAsyncContextEmpty(s.testAddress0)

		tokenId := types.TokenId(s.testAddress0)

		tokens, err := s.DefaultClient.GetTokens(s.Context, s.testAddress0, "latest")
		s.Require().NoError(err)
		s.Require().Len(tokens, 1)
		s.Equal(types.NewValueFromUint64(600_000), tokens[tokenId])

		tokens, err = s.DefaultClient.GetTokens(s.Context, s.counterAddress0, "latest")
		s.Require().NoError(err)
		s.Require().Len(tokens, 1)
		s.Equal(types.NewValueFromUint64(400_000), tokens[tokenId])
	})

	// TODO: support
	// s.Run("Out of gas response", func() {
	//	data := s.AbiPack(s.abiTest, "requestOutOfGasFailure", s.testAddress1)
	//	receipt := s.SendExternalTransactionNoCheck(data, s.testAddress0)
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

func (s *SuiteRequestResponse) TestOnlyResponse() {
	data := s.AbiPack(s.abiTest, "responseCounterAdd", true, []byte{}, []byte{})
	receipt := s.SendExternalTransactionNoCheck(data, s.testAddress0)
	s.Require().False(receipt.Success)
	s.Require().Equal("OnlyResponseCheckFailed", receipt.Status)
}

func (s *SuiteRequestResponse) checkAsyncContextEmpty(address types.Address) {
	s.T().Helper()

	contract := tests.GetContract(s.T(), s.DefaultClient, address)
	s.Require().Equal(common.EmptyHash, contract.AsyncContextRoot)
}

func TestRequestResponse(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteRequestResponse))
}
