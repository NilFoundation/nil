package rpctest

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/NilFoundation/nil/nil/internal/collate"
	"github.com/NilFoundation/nil/nil/internal/contracts"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/nilservice"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/stretchr/testify/suite"
)

type SuiteEconomy struct {
	RpcSuite
	walletAddress types.Address
	testAddress1  types.Address
	testAddress2  types.Address
	testAddress3  types.Address
	testAddress4  types.Address
	abiTest       *abi.ABI
	abiWallet     *abi.ABI
	zerostateCfg  string
	namesMap      map[types.Address]string
}

func (s *SuiteEconomy) SetupSuite() {
	s.shardsNum = 4

	var err error
	s.testAddress1, err = contracts.CalculateAddress(contracts.NameTest, 1, []byte{1})
	s.Require().NoError(err)
	s.testAddress2, err = contracts.CalculateAddress(contracts.NameTest, 2, []byte{2})
	s.Require().NoError(err)
	s.testAddress3, err = contracts.CalculateAddress(contracts.NameTest, 3, []byte{3})
	s.Require().NoError(err)
	s.testAddress4, err = contracts.CalculateAddress(contracts.NameTest, 1, []byte{4})
	s.Require().NoError(err)
	s.walletAddress = types.MainWalletAddress

	s.namesMap = map[types.Address]string{
		s.walletAddress: "wallet",
		s.testAddress1:  "test1",
		s.testAddress2:  "test2",
		s.testAddress3:  "test3",
		s.testAddress4:  "test4",
	}

	zerostateTmpl := `
config:
  gasPrices: [10, 10, 10, 50]
contracts:
- name: MainWallet
  address: {{ .WalletAddress }}
  value: 100000000000000
  contract: Wallet
  ctorArgs: [{{ .MainPublicKey }}]
- name: Test1
  address: {{ .TestAddress1 }}
  value: 10000000
  contract: tests/Test
- name: Test2
  address: {{ .TestAddress2 }}
  value: 0
  contract: tests/Test
- name: Test3
  address: {{ .TestAddress3 }}
  value: 0
  contract: tests/Test
- name: Test4
  address: {{ .TestAddress4 }}
  value: 0
  contract: tests/Test
`
	s.zerostateCfg, err = common.ParseTemplate(zerostateTmpl, map[string]any{
		"WalletAddress": s.walletAddress.Hex(),
		"MainPublicKey": hexutil.Encode(execution.MainPublicKey),
		"TestAddress1":  s.testAddress1.Hex(),
		"TestAddress2":  s.testAddress2.Hex(),
		"TestAddress3":  s.testAddress3.Hex(),
		"TestAddress4":  s.testAddress4.Hex(),
	})
	s.Require().NoError(err)

	s.abiWallet, err = contracts.GetAbi("Wallet")
	s.Require().NoError(err)

	s.abiTest, err = contracts.GetAbi(contracts.NameTest)
	s.Require().NoError(err)
}

func (s *SuiteEconomy) SetupTest() {
	s.start(&nilservice.Config{
		NShards:              s.shardsNum,
		HttpPort:             8537,
		Topology:             collate.TrivialShardTopologyId,
		ZeroState:            s.zerostateCfg,
		CollatorTickPeriodMs: 300,
		GasPriceScale:        0,
		GasBasePrice:         10,
		RunMode:              nilservice.CollatorsOnlyRunMode,
	})
}

func (s *SuiteEconomy) TearDownTest() {
	s.cancel()
}

func (s *SuiteEconomy) TestSeparateGasAndValue() {
	var (
		receipt        *jsonrpc.RPCReceipt
		data           []byte
		err            error
		info           ReceiptInfo
		initialBalance types.Value
		gasPrice       types.Value
	)
	initialBalance = s.getBalance(s.testAddress1).
		Add(s.getBalance(s.testAddress2)).
		Add(s.getBalance(s.testAddress3)).
		Add(s.getBalance(s.testAddress4)).
		Add(s.getBalance(s.walletAddress))

	// At first, test gas price getter.
	data, err = s.abiTest.Pack("getGasPrice")
	s.Require().NoError(err)

	retData := s.CallGetter(s.testAddress2, data, "latest", nil)
	unpackedRes, err := s.abiTest.Unpack("getGasPrice", retData)
	s.Require().NoError(err)
	gasPrice, err = s.client.GasPrice(s.testAddress2.ShardId())
	s.Require().NoError(err)
	s.Require().Equal(gasPrice.ToBig(), unpackedRes[0])

	retData = s.CallGetter(s.testAddress3, data, "latest", nil)
	unpackedRes, err = s.abiTest.Unpack("getGasPrice", retData)
	s.Require().NoError(err)
	gasPrice, err = s.client.GasPrice(s.testAddress3.ShardId())
	s.Require().NoError(err)
	s.Require().Equal(gasPrice.ToBig(), unpackedRes[0])

	// Call non-payable function with zero value. Success means that the fee is not debited from Value.
	data, err = s.abiTest.Pack("nonPayable")
	s.Require().NoError(err)

	receipt = s.sendMessageViaWalletNoCheck(s.walletAddress, s.testAddress1, execution.MainPrivateKey, data,
		s.gasToValue(100_000), types.NewValueFromUint64(0), nil)
	info = s.analyzeReceipt(receipt, s.namesMap)
	s.Require().True(info.AllSuccess())
	s.Require().True(info.ContainsOnly(s.walletAddress, s.testAddress1))
	initialBalance = s.checkBalance(info, initialBalance)

	// Call function that reverts. Bounced value should be equal to the value sent.
	data, err = s.abiTest.Pack("mayRevert", true)
	s.Require().NoError(err)
	receipt = s.sendMessageViaWalletNoCheck(s.walletAddress, s.testAddress1, execution.MainPrivateKey, data,
		s.gasToValue(100_000), types.NewValueFromUint64(1000), nil)
	info = s.analyzeReceipt(receipt, s.namesMap)
	s.Require().True(info[s.walletAddress].IsSuccess())
	s.Require().False(info[s.testAddress1].IsSuccess())
	s.Require().True(info.ContainsOnly(s.walletAddress, s.testAddress1))
	s.Require().Equal(types.NewValueFromUint64(1000), info[s.walletAddress].BounceReceived)
	s.Require().Equal(info[s.walletAddress].GetValueSpent(), info[s.testAddress1].ValueUsed)
	initialBalance = s.checkBalance(info, initialBalance)

	// Call sequence: wallet => test1 => test2. Where refundTo is wallet and bounceTo is test1.
	data, err = s.abiTest.Pack("noReturn")
	s.Require().NoError(err)
	data, err = s.abiTest.Pack("proxyCall", s.testAddress2, big.NewInt(1_000_000), big.NewInt(1_000_000),
		s.walletAddress, s.testAddress1, data)
	s.Require().NoError(err)
	receipt = s.sendMessageViaWalletNoCheck(s.walletAddress, s.testAddress1, execution.MainPrivateKey, data,
		s.gasToValue(1_000_000), types.NewValueFromUint64(2_000_000), nil)
	info = s.analyzeReceipt(receipt, s.namesMap)
	s.Require().True(info.AllSuccess())
	s.Require().True(info.ContainsOnly(s.walletAddress, s.testAddress1, s.testAddress2))
	s.Require().Zero(info[s.testAddress1].RefundReceived)
	initialBalance = s.checkBalance(info, initialBalance)

	// Call sequence: wallet => test1 => test2. Where bounceTo and refundTo is equal to test1.
	data, err = s.abiTest.Pack("mayRevert", true)
	s.Require().NoError(err)
	data, err = s.abiTest.Pack("proxyCall", s.testAddress2, big.NewInt(1_000_000), big.NewInt(1_000_000),
		s.testAddress1, s.testAddress1, data)
	s.Require().NoError(err)
	receipt = s.sendMessageViaWalletNoCheck(s.walletAddress, s.testAddress1, execution.MainPrivateKey, data,
		s.gasToValue(1_000_000), types.NewValueFromUint64(2_000_000), nil)
	info = s.analyzeReceipt(receipt, s.namesMap)
	s.Require().True(info.ContainsOnly(s.walletAddress, s.testAddress1, s.testAddress2))
	s.Require().True(info[s.walletAddress].IsSuccess())
	s.Require().True(info[s.testAddress1].IsSuccess())
	s.Require().False(info[s.testAddress2].IsSuccess())
	initialBalance = s.checkBalance(info, initialBalance)

	// Call sequence: wallet => test1 => test2. Where refundTo=wallet and bounceTo=test1.
	// So after bounce is processed, leftover gas should be refunded to wallet.
	data, err = s.abiTest.Pack("mayRevert", true)
	s.Require().NoError(err)
	data, err = s.abiTest.Pack("proxyCall", s.testAddress2, big.NewInt(1_000_000), big.NewInt(1_000_000),
		s.walletAddress, s.testAddress1, data)
	s.Require().NoError(err)
	receipt = s.sendMessageViaWalletNoCheck(s.walletAddress, s.testAddress1, execution.MainPrivateKey, data,
		s.gasToValue(1_000_000), types.NewValueFromUint64(2_000_000), nil)
	s.Require().True(receipt.Success)
	info = s.analyzeReceipt(receipt, s.namesMap)
	s.Require().True(info[s.walletAddress].IsSuccess())
	s.Require().True(info[s.testAddress1].IsSuccess())
	s.Require().False(info[s.testAddress2].IsSuccess())
	s.Require().Zero(info[s.testAddress1].RefundReceived.Cmp(types.NewValueFromUint64(0)))
	s.Require().Positive(info[s.walletAddress].RefundReceived.Cmp(types.NewValueFromUint64(1_000_000)))
	s.checkBalance(info, initialBalance)
}

type AsyncCallArgs struct {
	Addr        types.Address
	FeeCredit   *big.Int
	ForwardKind uint8
	RefundTo    types.Address
	CallData    []byte
}

func (s *SuiteEconomy) TestGasForwarding() { //nolint
	var (
		receipt        *jsonrpc.RPCReceipt
		data           []byte
		info           ReceiptInfo
		initialBalance types.Value
	)

	unpackStubEvent := func(receipt *jsonrpc.RPCReceipt) uint32 {
		a, err := s.abiTest.Events["stubCalled"].Inputs.Unpack(receipt.Logs[0].Data)
		s.Require().NoError(err)
		res, ok := a[0].(uint32)
		s.Require().True(ok)
		return res
	}

	initialBalance = s.getBalance(s.testAddress1).
		Add(s.getBalance(s.testAddress2)).
		Add(s.getBalance(s.testAddress3)).
		Add(s.getBalance(s.testAddress4)).
		Add(s.getBalance(s.walletAddress))

	args := make([]AsyncCallArgs, 0, 10)

	// w -> t1 -> {t2[rem]}
	args = args[:0]
	args = append(args, AsyncCallArgs{
		Addr:        s.testAddress2,
		FeeCredit:   big.NewInt(0),
		ForwardKind: types.ForwardKindRemaining,
		CallData:    s.AbiPack(s.abiTest, "stub", big.NewInt(1)),
	})
	data = s.AbiPack(s.abiTest, "testForwarding", args)
	receipt = s.sendMessageViaWalletNoCheck(s.walletAddress, s.testAddress1, execution.MainPrivateKey, data,
		s.gasToValue(100_000), types.NewValueFromUint64(0), nil)
	info = s.analyzeReceipt(receipt, s.namesMap)
	s.Require().True(info.AllSuccess())
	s.Require().True(info.ContainsOnly(s.walletAddress, s.testAddress1, s.testAddress2))
	initialBalance = s.checkBalance(info, initialBalance)

	// w -> t1 -> {t2[no]}: no forwarding, all gas is refunded
	args = args[:0]
	args = append(args, AsyncCallArgs{
		Addr:        s.testAddress2,
		FeeCredit:   big.NewInt(1_000_000),
		ForwardKind: types.ForwardKindNone,
		RefundTo:    s.walletAddress,
		CallData:    s.AbiPack(s.abiTest, "stub", big.NewInt(1)),
	})
	data = s.AbiPack(s.abiTest, "testForwarding", args)
	receipt = s.sendMessageViaWalletNoCheck(s.walletAddress, s.testAddress1, execution.MainPrivateKey, data,
		s.gasToValue(100_000), types.NewValueFromUint64(0), nil)
	info = s.analyzeReceipt(receipt, s.namesMap)
	s.Require().True(info.AllSuccess())
	s.Require().True(info.ContainsOnly(s.walletAddress, s.testAddress1, s.testAddress2))
	s.Require().True(info[s.testAddress1].ValueForwarded.IsZero())
	initialBalance = s.checkBalance(info, initialBalance)

	// w -> t1 -> {t2[percent]}: refund rest from t1
	args = args[:0]
	args = append(args, AsyncCallArgs{
		Addr:        s.testAddress2,
		FeeCredit:   big.NewInt(70),
		ForwardKind: types.ForwardKindPercentage,
		CallData:    s.AbiPack(s.abiTest, "stub", big.NewInt(1)),
	})
	data = s.AbiPack(s.abiTest, "testForwarding", args)
	receipt = s.sendMessageViaWalletNoCheck(s.walletAddress, s.testAddress1, execution.MainPrivateKey, data,
		s.gasToValue(100_000), types.NewValueFromUint64(0), nil)
	info = s.analyzeReceipt(receipt, s.namesMap)
	s.Require().True(info.AllSuccess())
	s.Require().True(info.ContainsOnly(s.walletAddress, s.testAddress1, s.testAddress2))
	s.Require().False(info[s.testAddress1].ValueForwarded.IsZero())
	s.Require().False(info[s.testAddress2].RefundSent.IsZero())
	initialBalance = s.checkBalance(info, initialBalance)

	// w -> t1 -> {t2[percent], t3[no]}: no forward for t3, fee is debited from account
	args = args[:0]
	args = append(args, AsyncCallArgs{
		Addr:        s.testAddress2,
		FeeCredit:   big.NewInt(70),
		ForwardKind: types.ForwardKindPercentage,
		CallData:    s.AbiPack(s.abiTest, "stub", big.NewInt(1)),
	})
	args = append(args, AsyncCallArgs{
		Addr:        s.testAddress3,
		FeeCredit:   big.NewInt(100_000),
		ForwardKind: types.ForwardKindNone,
		CallData:    s.AbiPack(s.abiTest, "stub", big.NewInt(2)),
	})
	data = s.AbiPack(s.abiTest, "testForwarding", args)
	receipt = s.sendMessageViaWalletNoCheck(s.walletAddress, s.testAddress1, execution.MainPrivateKey, data,
		s.gasToValue(100_000), types.NewValueFromUint64(0), nil)
	info = s.analyzeReceipt(receipt, s.namesMap)
	s.Require().True(info.AllSuccess())
	s.Require().True(info.ContainsOnly(s.walletAddress, s.testAddress1, s.testAddress2, s.testAddress3))
	s.Require().False(info[s.testAddress1].ValueForwarded.IsZero())
	s.Require().False(info[s.testAddress2].RefundSent.IsZero())
	s.Require().Equal(info[s.testAddress1].ValueForwarded, info[s.testAddress2].ValueUsed.Add(info[s.testAddress2].RefundSent))
	initialBalance = s.checkBalance(info, initialBalance)

	// w -> t1 -> {t2[val]}: refund rest from t1
	args = args[:0]
	args = append(args, AsyncCallArgs{
		Addr:        s.testAddress2,
		FeeCredit:   big.NewInt(200_000),
		ForwardKind: types.ForwardKindValue,
		CallData:    s.AbiPack(s.abiTest, "stub", big.NewInt(1)),
	})
	data = s.AbiPack(s.abiTest, "testForwarding", args)
	receipt = s.sendMessageViaWalletNoCheck(s.walletAddress, s.testAddress1, execution.MainPrivateKey, data,
		s.gasToValue(100_000), types.NewValueFromUint64(0), nil)
	info = s.analyzeReceipt(receipt, s.namesMap)
	s.Require().True(info.AllSuccess())
	s.Require().True(info.ContainsOnly(s.walletAddress, s.testAddress1, s.testAddress2))
	s.Require().Equal(0, info[s.testAddress1].ValueForwarded.Cmp(types.NewValueFromUint64(200_000)))
	s.Require().False(info[s.testAddress1].RefundSent.IsZero())
	s.Require().Equal(info[s.walletAddress].ValueSent, info[s.testAddress1].ValueForwarded.
		Add(info[s.testAddress1].ValueUsed).Add(info[s.testAddress1].RefundSent))
	initialBalance = s.checkBalance(info, initialBalance)

	// w -> t1 -> {t2[val], t3[percent]}: refund rest from t1
	args = args[:0]
	args = append(args, AsyncCallArgs{
		Addr:        s.testAddress2,
		FeeCredit:   big.NewInt(200_000),
		ForwardKind: types.ForwardKindValue,
		CallData:    s.AbiPack(s.abiTest, "stub", big.NewInt(1)),
	})
	args = append(args, AsyncCallArgs{
		Addr:        s.testAddress3,
		FeeCredit:   big.NewInt(60),
		ForwardKind: types.ForwardKindPercentage,
		CallData:    s.AbiPack(s.abiTest, "stub", big.NewInt(2)),
	})
	data = s.AbiPack(s.abiTest, "testForwarding", args)
	receipt = s.sendMessageViaWalletNoCheck(s.walletAddress, s.testAddress1, execution.MainPrivateKey, data,
		s.gasToValue(100_000), types.NewValueFromUint64(0), nil)
	info = s.analyzeReceipt(receipt, s.namesMap)
	s.Require().True(info.AllSuccess())
	s.Require().True(info.ContainsOnly(s.walletAddress, s.testAddress1, s.testAddress2, s.testAddress3))
	s.Require().False(info[s.testAddress1].ValueForwarded.IsZero())
	s.Require().False(info[s.testAddress1].RefundSent.IsZero())
	s.Require().Equal(info[s.walletAddress].ValueSent, info[s.testAddress1].ValueForwarded.
		Add(info[s.testAddress1].ValueUsed).Add(info[s.testAddress1].RefundSent))
	initialBalance = s.checkBalance(info, initialBalance)

	// w -> t1 -> {t2[val], t3[percent], t4[rem]}: no refund from t1
	args = args[:0]
	args = append(args, AsyncCallArgs{
		Addr:        s.testAddress2,
		FeeCredit:   big.NewInt(200_000),
		ForwardKind: types.ForwardKindValue,
		CallData:    s.AbiPack(s.abiTest, "stub", big.NewInt(1)),
	})
	args = append(args, AsyncCallArgs{
		Addr:        s.testAddress3,
		FeeCredit:   big.NewInt(60),
		ForwardKind: types.ForwardKindPercentage,
		CallData:    s.AbiPack(s.abiTest, "stub", big.NewInt(2)),
	})
	args = append(args, AsyncCallArgs{
		Addr:        s.testAddress4,
		FeeCredit:   big.NewInt(123),
		ForwardKind: types.ForwardKindRemaining,
		CallData:    s.AbiPack(s.abiTest, "stub", big.NewInt(3)),
	})
	data = s.AbiPack(s.abiTest, "testForwarding", args)
	receipt = s.sendMessageViaWalletNoCheck(s.walletAddress, s.testAddress1, execution.MainPrivateKey, data,
		s.gasToValue(100_000), types.NewValueFromUint64(0), nil)
	info = s.analyzeReceipt(receipt, s.namesMap)
	s.Require().True(info.AllSuccess())
	s.Require().True(info.ContainsOnly(s.walletAddress, s.testAddress1, s.testAddress2, s.testAddress3, s.testAddress4))
	s.Require().Equal(uint32(1), unpackStubEvent(receipt.OutReceipts[0].OutReceipts[0]))
	s.Require().Equal(uint32(2), unpackStubEvent(receipt.OutReceipts[0].OutReceipts[1]))
	s.Require().Equal(uint32(3), unpackStubEvent(receipt.OutReceipts[0].OutReceipts[2]))
	s.Require().False(info[s.testAddress1].ValueForwarded.IsZero())
	s.Require().True(info[s.testAddress1].RefundSent.IsZero())
	s.Require().Equal(info[s.walletAddress].ValueSent, info[s.testAddress1].ValueForwarded.
		Add(info[s.testAddress1].ValueUsed))
	initialBalance = s.checkBalance(info, initialBalance)

	// w -> t1 -> {t2[percent], t3[percent], t4[rem]}: percent is not 100%, so there is enough for remaining forwarding
	args = args[:0]
	args = append(args, AsyncCallArgs{
		Addr:        s.testAddress2,
		FeeCredit:   big.NewInt(30),
		ForwardKind: types.ForwardKindPercentage,
		CallData:    s.AbiPack(s.abiTest, "stub", big.NewInt(1)),
	})
	args = append(args, AsyncCallArgs{
		Addr:        s.testAddress3,
		FeeCredit:   big.NewInt(40),
		ForwardKind: types.ForwardKindPercentage,
		CallData:    s.AbiPack(s.abiTest, "stub", big.NewInt(2)),
	})
	args = append(args, AsyncCallArgs{
		Addr:        s.testAddress4,
		FeeCredit:   big.NewInt(123),
		ForwardKind: types.ForwardKindRemaining,
		CallData:    s.AbiPack(s.abiTest, "stub", big.NewInt(3)),
	})
	data = s.AbiPack(s.abiTest, "testForwarding", args)
	receipt = s.sendMessageViaWalletNoCheck(s.walletAddress, s.testAddress1, execution.MainPrivateKey, data,
		s.gasToValue(100_000), types.NewValueFromUint64(0), nil)
	info = s.analyzeReceipt(receipt, s.namesMap)
	s.Require().True(info.AllSuccess())
	s.Require().True(info.ContainsOnly(s.walletAddress, s.testAddress1, s.testAddress2, s.testAddress3, s.testAddress4))
	s.Require().Equal(uint32(1), unpackStubEvent(receipt.OutReceipts[0].OutReceipts[0]))
	s.Require().Equal(uint32(2), unpackStubEvent(receipt.OutReceipts[0].OutReceipts[1]))
	s.Require().Equal(uint32(3), unpackStubEvent(receipt.OutReceipts[0].OutReceipts[2]))
	s.Require().False(info[s.testAddress1].ValueForwarded.IsZero())
	s.Require().True(info[s.testAddress1].RefundSent.IsZero())
	s.Require().Equal(info[s.walletAddress].ValueSent, info[s.testAddress1].ValueForwarded.
		Add(info[s.testAddress1].ValueUsed))
	initialBalance = s.checkBalance(info, initialBalance)

	// w -> t1 -> {t2[percent], t3[percent], t4[rem]}: percent is 100%, should fail since no gas for t4
	args = args[:0]
	args = append(args, AsyncCallArgs{
		Addr:        s.testAddress2,
		FeeCredit:   big.NewInt(60),
		ForwardKind: types.ForwardKindPercentage,
		CallData:    s.AbiPack(s.abiTest, "stub", big.NewInt(1)),
	})
	args = append(args, AsyncCallArgs{
		Addr:        s.testAddress3,
		FeeCredit:   big.NewInt(40),
		ForwardKind: types.ForwardKindPercentage,
		CallData:    s.AbiPack(s.abiTest, "stub", big.NewInt(2)),
	})
	args = append(args, AsyncCallArgs{
		Addr:        s.testAddress4,
		FeeCredit:   big.NewInt(123),
		ForwardKind: types.ForwardKindRemaining,
		CallData:    s.AbiPack(s.abiTest, "stub", big.NewInt(3)),
	})
	data = s.AbiPack(s.abiTest, "testForwarding", args)
	receipt = s.sendMessageViaWalletNoCheck(s.walletAddress, s.testAddress1, execution.MainPrivateKey, data,
		s.gasToValue(100_000), types.NewValueFromUint64(0), nil)
	info = s.analyzeReceipt(receipt, s.namesMap)
	s.Require().False(info.AllSuccess())
	s.Require().True(info.ContainsOnly(s.walletAddress, s.testAddress1, s.testAddress2, s.testAddress3, s.testAddress4))
	s.Require().False(info[s.testAddress1].ValueForwarded.IsZero())
	s.Require().True(info[s.testAddress1].RefundSent.IsZero())
	s.Require().True(info[s.testAddress4].ValueUsed.IsZero())
	s.Require().Equal(info[s.walletAddress].ValueSent, info[s.testAddress1].ValueForwarded.Add(info[s.testAddress1].RefundSent).
		Add(info[s.testAddress1].ValueUsed))
	initialBalance = s.checkBalance(info, initialBalance)

	// w -> t1 -> {t2[percent], t3[percent]}: fail - percentage is more than 100%
	args = args[:0]
	args = append(args, AsyncCallArgs{
		Addr:        s.testAddress2,
		FeeCredit:   big.NewInt(60),
		ForwardKind: types.ForwardKindPercentage,
		CallData:    s.AbiPack(s.abiTest, "stub", big.NewInt(1)),
	})
	args = append(args, AsyncCallArgs{
		Addr:        s.testAddress3,
		FeeCredit:   big.NewInt(50),
		ForwardKind: types.ForwardKindPercentage,
		CallData:    s.AbiPack(s.abiTest, "stub", big.NewInt(2)),
	})
	data = s.AbiPack(s.abiTest, "testForwarding", args)
	receipt = s.sendMessageViaWalletNoCheck(s.walletAddress, s.testAddress1, execution.MainPrivateKey, data,
		s.gasToValue(100_000), types.NewValueFromUint64(0), nil)
	info = s.analyzeReceipt(receipt, s.namesMap)
	s.Require().False(info.AllSuccess())
	s.Require().True(info.ContainsOnly(s.walletAddress, s.testAddress1))
	s.Require().True(info[s.testAddress1].ValueForwarded.IsZero())
	s.Require().False(info[s.testAddress1].RefundSent.IsZero())
	s.Require().True(info[s.testAddress1].ValueSent.IsZero())
	s.Require().Equal(info[s.walletAddress].ValueSent, info[s.testAddress1].RefundSent.Add(info[s.testAddress1].ValueUsed))
	initialBalance = s.checkBalance(info, initialBalance)

	// w -> t1 -> {t2[percent], t3[rem], t4[rem]}: equal parts, no refund
	args = args[:0]
	args = append(args, AsyncCallArgs{
		Addr:        s.testAddress2,
		FeeCredit:   big.NewInt(40),
		ForwardKind: types.ForwardKindPercentage,
		CallData:    s.AbiPack(s.abiTest, "stub", big.NewInt(1)),
	})
	args = append(args, AsyncCallArgs{
		Addr:        s.testAddress3,
		FeeCredit:   big.NewInt(123),
		ForwardKind: types.ForwardKindRemaining,
		CallData:    s.AbiPack(s.abiTest, "stub", big.NewInt(2)),
	})
	args = append(args, AsyncCallArgs{
		Addr:        s.testAddress4,
		FeeCredit:   big.NewInt(456),
		ForwardKind: types.ForwardKindRemaining,
		CallData:    s.AbiPack(s.abiTest, "stub", big.NewInt(3)),
	})
	data = s.AbiPack(s.abiTest, "testForwarding", args)
	receipt = s.sendMessageViaWalletNoCheck(s.walletAddress, s.testAddress1, execution.MainPrivateKey, data,
		s.gasToValue(100_000), types.NewValueFromUint64(0), nil)
	info = s.analyzeReceipt(receipt, s.namesMap)
	s.Require().True(info.AllSuccess())
	s.Require().True(info.ContainsOnly(s.walletAddress, s.testAddress1, s.testAddress2, s.testAddress3, s.testAddress4))
	s.Require().False(info[s.testAddress1].ValueForwarded.IsZero())
	s.Require().True(info[s.testAddress1].RefundSent.IsZero())
	// Check test3 and test4 get same fee credit
	s.Require().Equal(info[s.testAddress1].OutMessages[s.testAddress3].FeeCredit,
		info[s.testAddress1].OutMessages[s.testAddress4].FeeCredit)
	s.Require().Equal(info[s.walletAddress].ValueSent,
		info[s.testAddress1].ValueForwarded.
			Add(info[s.testAddress1].ValueUsed))
	initialBalance = s.checkBalance(info, initialBalance)

	// w -> t1 -> {t2[percent, refundTo=t3], t3[rem, refundTo=t2]}: specify refundTo
	args = args[:0]
	args = append(args, AsyncCallArgs{
		Addr:        s.testAddress2,
		RefundTo:    s.testAddress3,
		FeeCredit:   big.NewInt(70),
		ForwardKind: types.ForwardKindPercentage,
		CallData:    s.AbiPack(s.abiTest, "stub", big.NewInt(1)),
	})
	args = append(args, AsyncCallArgs{
		Addr:        s.testAddress3,
		RefundTo:    s.testAddress2,
		FeeCredit:   big.NewInt(123),
		ForwardKind: types.ForwardKindRemaining,
		CallData:    s.AbiPack(s.abiTest, "stub", big.NewInt(2)),
	})
	data = s.AbiPack(s.abiTest, "testForwarding", args)
	receipt = s.sendMessageViaWalletNoCheck(s.walletAddress, s.testAddress1, execution.MainPrivateKey, data,
		s.gasToValue(100_000), types.NewValueFromUint64(0), nil)
	info = s.analyzeReceipt(receipt, s.namesMap)
	s.Require().True(info.AllSuccess())
	s.Require().True(info.ContainsOnly(s.walletAddress, s.testAddress1, s.testAddress2, s.testAddress3))
	s.Require().False(info[s.testAddress1].ValueForwarded.IsZero())
	s.Require().False(info[s.testAddress2].RefundSent.IsZero())
	s.Require().Equal(info[s.testAddress2].RefundSent, info[s.testAddress3].RefundReceived)
	s.Require().Equal(info[s.testAddress2].RefundReceived, info[s.testAddress3].RefundSent)
	initialBalance = s.checkBalance(info, initialBalance)

	// t1 -> {t2[rem]}: forward from external message
	args = args[:0]
	args = append(args, AsyncCallArgs{
		Addr:        s.testAddress2,
		FeeCredit:   big.NewInt(0),
		ForwardKind: types.ForwardKindRemaining,
		CallData:    s.AbiPack(s.abiTest, "stub", big.NewInt(1)),
	})
	data = s.AbiPack(s.abiTest, "testForwarding", args)
	receipt = s.sendExternalMessage(data, s.testAddress1)
	info = s.analyzeReceipt(receipt, s.namesMap)
	s.Require().True(info.AllSuccess())
	s.Require().True(info.ContainsOnly(s.testAddress1, s.testAddress2))
	initialBalance = s.checkBalance(info, initialBalance)

	// t1 -> {t2[rem]}: fail - forward too much from external message, should correctly refund to account
	args = args[:0]
	args = append(args, AsyncCallArgs{
		Addr:        s.testAddress2,
		FeeCredit:   big.NewInt(101),
		ForwardKind: types.ForwardKindPercentage,
		CallData:    s.AbiPack(s.abiTest, "stub", big.NewInt(1)),
	})
	data = s.AbiPack(s.abiTest, "testForwarding", args)
	receipt = s.sendExternalMessageNoCheck(data, s.testAddress1)
	info = s.analyzeReceipt(receipt, s.namesMap)
	s.Require().False(info.AllSuccess())
	s.Require().True(info.ContainsOnly(s.testAddress1))
	initialBalance = s.checkBalance(info, initialBalance)

	// w -> t1 -> {t2[val]}: fail - val is greater than available feeCredit
	args = args[:0]
	args = append(args, AsyncCallArgs{
		Addr:        s.testAddress2,
		FeeCredit:   big.NewInt(1_000_000_000_000),
		ForwardKind: types.ForwardKindValue,
		CallData:    s.AbiPack(s.abiTest, "stub", big.NewInt(1)),
	})
	data = s.AbiPack(s.abiTest, "testForwarding", args)
	receipt = s.sendMessageViaWalletNoCheck(s.walletAddress, s.testAddress1, execution.MainPrivateKey, data,
		s.gasToValue(100_000), types.NewValueFromUint64(0), nil)
	info = s.analyzeReceipt(receipt, s.namesMap)
	s.Require().False(info.AllSuccess())
	s.Require().True(info.ContainsOnly(s.walletAddress, s.testAddress1))
	s.checkBalance(info, initialBalance)
}

// TestForwardKindMatch checks that types.ForwardKind matches to forward kinds from `Nil.sol`
func (s *SuiteEconomy) TestForwardKindMatch() {
	var data []byte
	var err error

	data, err = s.abiTest.Pack("getForwardKindRemaining")
	s.Require().NoError(err)
	data = s.CallGetter(s.testAddress2, data, "latest", nil)
	unpackedRes, err := s.abiTest.Unpack("getForwardKindRemaining", data)
	s.Require().NoError(err)
	s.Require().Equal(uint8(types.ForwardKindRemaining), unpackedRes[0])

	data, err = s.abiTest.Pack("getForwardKindPercentage")
	s.Require().NoError(err)
	data = s.CallGetter(s.testAddress2, data, "latest", nil)
	unpackedRes, err = s.abiTest.Unpack("getForwardKindPercentage", data)
	s.Require().NoError(err)
	s.Require().Equal(uint8(types.ForwardKindPercentage), unpackedRes[0])

	data, err = s.abiTest.Pack("getForwardKindValue")
	s.Require().NoError(err)
	data = s.CallGetter(s.testAddress2, data, "latest", nil)
	unpackedRes, err = s.abiTest.Unpack("getForwardKindValue", data)
	s.Require().NoError(err)
	s.Require().Equal(uint8(types.ForwardKindValue), unpackedRes[0])

	data, err = s.abiTest.Pack("getForwardKindNone")
	s.Require().NoError(err)
	data = s.CallGetter(s.testAddress2, data, "latest", nil)
	unpackedRes, err = s.abiTest.Unpack("getForwardKindNone", data)
	s.Require().NoError(err)
	s.Require().Equal(uint8(types.ForwardKindNone), unpackedRes[0])
}

func (s *SuiteEconomy) checkBalance(infoMap ReceiptInfo, balance types.Value) types.Value {
	s.T().Helper()

	newBalance := s.getBalance(s.testAddress1).
		Add(s.getBalance(s.testAddress2)).
		Add(s.getBalance(s.testAddress3)).
		Add(s.getBalance(s.testAddress4)).
		Add(s.getBalance(s.walletAddress))

	newRealBalance := newBalance

	fmt.Printf("newRealBalance: %v\n", newRealBalance)

	for _, info := range infoMap {
		newBalance = newBalance.Add(info.ValueUsed)
	}
	s.Require().Equal(balance, newBalance)

	return newRealBalance
}

func TestEconomyRpc(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteEconomy))
}
