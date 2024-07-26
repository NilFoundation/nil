package rpctest

import (
	"math/big"
	"testing"

	"github.com/NilFoundation/nil/cmd/nil/nilservice"
	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/hexutil"
	"github.com/NilFoundation/nil/contracts"
	"github.com/NilFoundation/nil/core/collate"
	"github.com/NilFoundation/nil/core/execution"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/rpc/jsonrpc"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/stretchr/testify/suite"
)

type SuiteEconomy struct {
	RpcSuite
	walletAddress types.Address
	testAddress1  types.Address
	testAddress2  types.Address
	testAddress3  types.Address
	abiTest       *abi.ABI
	abiWallet     *abi.ABI
	zerostateCfg  string
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

	s.walletAddress = types.MainWalletAddress

	zerostateTmpl := `
config:
  gas_price: [10, 10, 10, 50]
contracts:
- name: MainWallet
  address: {{ .WalletAddress }}
  value: 100000000000000
  contract: Wallet
  ctorArgs: [{{ .MainPublicKey }}]
- name: Test1
  address: {{ .TestAddress1 }}
  value: 0
  contract: tests/Test
- name: Test2
  address: {{ .TestAddress2 }}
  value: 0
  contract: tests/Test
- name: Test3
  address: {{ .TestAddress3 }}
  value: 0
  contract: tests/Test
`
	s.zerostateCfg, err = common.ParseTemplate(zerostateTmpl, map[string]any{
		"WalletAddress": s.walletAddress.Hex(),
		"MainPublicKey": hexutil.Encode(execution.MainPublicKey),
		"TestAddress1":  s.testAddress1.Hex(),
		"TestAddress2":  s.testAddress2.Hex(),
		"TestAddress3":  s.testAddress3.Hex(),
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
		HttpPort:             8534,
		Topology:             collate.TrivialShardTopologyId,
		ZeroState:            s.zerostateCfg,
		CollatorTickPeriodMs: 100,
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
	initialBalance = s.getBalance(s.walletAddress)

	// At first, test gas price getter.
	data, err = s.abiTest.Pack("getGasPrice")
	s.Require().NoError(err)

	retData := s.CallGetter(s.testAddress2, data)
	unpackedRes, err := s.abiTest.Unpack("getGasPrice", hexutil.FromHex(string(retData)))
	s.Require().NoError(err)
	gasPrice, err = s.client.GasPrice(s.testAddress2.ShardId())
	s.Require().NoError(err)
	s.Require().Equal(gasPrice.ToBig(), unpackedRes[0])

	retData = s.CallGetter(s.testAddress3, data)
	unpackedRes, err = s.abiTest.Unpack("getGasPrice", hexutil.FromHex(string(retData)))
	s.Require().NoError(err)
	gasPrice, err = s.client.GasPrice(s.testAddress3.ShardId())
	s.Require().NoError(err)
	s.Require().Equal(gasPrice.ToBig(), unpackedRes[0])

	// Call non-payable function with zero value. Success means that the fee is not debited from Value.
	data, err = s.abiTest.Pack("nonPayable")
	s.Require().NoError(err)

	receipt = s.sendMessageViaWalletNoCheck(s.walletAddress, s.testAddress1, execution.MainPrivateKey, data,
		s.gasToValue(100_000), types.NewValueFromUint64(0), nil)
	info = s.analyzeReceipt(receipt)
	s.Require().True(info.AllSuccess())
	s.Require().True(info.ContainsOnly(s.walletAddress, s.testAddress1))
	initialBalance = s.checkBalance(info, initialBalance)

	// Call function that reverts. Bounced value should be equal to the value sent.
	data, err = s.abiTest.Pack("mayRevert", true)
	s.Require().NoError(err)
	receipt = s.sendMessageViaWalletNoCheck(s.walletAddress, s.testAddress1, execution.MainPrivateKey, data,
		s.gasToValue(100_000), types.NewValueFromUint64(1000), nil)
	info = s.analyzeReceipt(receipt)
	s.Require().True(info[s.walletAddress].IsSuccess())
	s.Require().False(info[s.testAddress1].IsSuccess())
	s.Require().True(info.ContainsOnly(s.walletAddress, s.testAddress1))
	s.Require().Equal(types.NewValueFromUint64(1000), info[s.walletAddress].ValueBounced)
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
	info = s.analyzeReceipt(receipt)
	s.Require().True(info.AllSuccess())
	s.Require().True(info.ContainsOnly(s.walletAddress, s.testAddress1, s.testAddress2))
	s.Require().Zero(info[s.testAddress1].ValueRefunded)
	initialBalance = s.checkBalance(info, initialBalance)

	// Call sequence: wallet => test1 => test2. Where bounceTo and refundTo is equal to test1.
	data, err = s.abiTest.Pack("mayRevert", true)
	s.Require().NoError(err)
	data, err = s.abiTest.Pack("proxyCall", s.testAddress2, big.NewInt(1_000_000), big.NewInt(1_000_000),
		s.testAddress1, s.testAddress1, data)
	s.Require().NoError(err)
	receipt = s.sendMessageViaWalletNoCheck(s.walletAddress, s.testAddress1, execution.MainPrivateKey, data,
		s.gasToValue(1_000_000), types.NewValueFromUint64(2_000_000), nil)
	info = s.analyzeReceipt(receipt)
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
	info = s.analyzeReceipt(receipt)
	s.Require().True(info[s.walletAddress].IsSuccess())
	s.Require().True(info[s.testAddress1].IsSuccess())
	s.Require().False(info[s.testAddress2].IsSuccess())
	s.Require().Zero(info[s.testAddress1].ValueRefunded.Cmp(types.NewValueFromUint64(0)))
	s.Require().Positive(info[s.walletAddress].ValueRefunded.Cmp(types.NewValueFromUint64(1_000_000)))
	s.checkBalance(info, initialBalance)
}

func (s *SuiteEconomy) checkBalance(infoMap ReceiptInfo, balance types.Value) types.Value {
	s.T().Helper()

	newBalance := s.getBalance(s.testAddress1).Add(s.getBalance(s.testAddress2).Add(s.getBalance(s.testAddress3)))
	newBalance = newBalance.Add(s.getBalance(s.walletAddress))

	newInitialBalance := newBalance

	for _, info := range infoMap {
		newBalance = newBalance.Add(info.ValueUsed)
	}
	s.Require().Equal(balance, newBalance)

	return newInitialBalance
}

func TestEconomyRpc(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteEconomy))
}
