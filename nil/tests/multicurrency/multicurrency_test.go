package main

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/NilFoundation/nil/nil/internal/abi"
	"github.com/NilFoundation/nil/nil/internal/contracts"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/internal/vm"
	"github.com/NilFoundation/nil/nil/services/nilservice"
	"github.com/NilFoundation/nil/nil/services/rpc"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
	"github.com/NilFoundation/nil/nil/tests"
	"github.com/stretchr/testify/suite"
)

type SuiteMultiCurrencyRpc struct {
	tests.RpcSuite
	walletAddress1      types.Address
	walletAddress2      types.Address
	walletAddress3      types.Address
	testAddress1_0      types.Address
	testAddress1_1      types.Address
	testAddressNoAccess types.Address
	abiTest             *abi.ABI
	abiWallet           *abi.ABI
	zerostateCfg        string
}

func (s *SuiteMultiCurrencyRpc) SetupSuite() {
	s.ShardsNum = 4

	s.walletAddress1 = contracts.WalletAddress(s.T(), 2, []byte{0}, execution.MainPublicKey)
	s.walletAddress2 = contracts.WalletAddress(s.T(), 3, []byte{1}, execution.MainPublicKey)
	s.walletAddress3 = contracts.WalletAddress(s.T(), 3, []byte{3}, execution.MainPublicKey)

	var err error
	s.testAddress1_0, err = contracts.CalculateAddress(contracts.NameTokensTest, 1, []byte{1})
	s.Require().NoError(err)

	s.testAddress1_1, err = contracts.CalculateAddress(contracts.NameTokensTest, 1, []byte{2})
	s.Require().NoError(err)

	s.testAddressNoAccess, err = contracts.CalculateAddress(contracts.NameTokensTestNoExternalAccess, 1, nil)
	s.Require().NoError(err)

	zerostateTmpl := `
contracts:
- name: TestWalletShard2
  address: {{ .TestAddress1 }}
  value: 100000000000000
  contract: Wallet
  ctorArgs: [{{ .MainPublicKey }}]
- name: TestWalletShard3
  address: {{ .TestAddress2 }}
  value: 100000000000000
  contract: Wallet
  ctorArgs: [{{ .MainPublicKey }}]
- name: TestWalletShard3a
  address: {{ .TestAddress3 }}
  value: 100000000000000
  contract: Wallet
  ctorArgs: [{{ .MainPublicKey }}]
- name: TokensTest1_0
  address: {{ .TokensTestAddress1_0 }}
  value: 100000000000000
  contract: tests/TokensTest
- name: TokensTest1_1
  address: {{ .TokensTestAddress1_1 }}
  value: 100000000000000
  contract: tests/TokensTest
- name: TokensTestNoAccess
  address: {{ .TokensTestNoAccess }}
  value: 100000000000000
  contract: tests/TokensTestNoExternalAccess
`
	s.zerostateCfg, err = common.ParseTemplate(zerostateTmpl, map[string]interface{}{
		"MainPublicKey":        hexutil.Encode(execution.MainPublicKey),
		"TestAddress1":         s.walletAddress1.Hex(),
		"TestAddress2":         s.walletAddress2.Hex(),
		"TestAddress3":         s.walletAddress3.Hex(),
		"TokensTestAddress1_0": s.testAddress1_0.Hex(),
		"TokensTestAddress1_1": s.testAddress1_1.Hex(),
		"TokensTestNoAccess":   s.testAddressNoAccess.Hex(),
	})
	s.Require().NoError(err)

	s.abiWallet, err = contracts.GetAbi("Wallet")
	s.Require().NoError(err)

	s.abiTest, err = contracts.GetAbi(contracts.NameTokensTest)
	s.Require().NoError(err)
}

func (s *SuiteMultiCurrencyRpc) SetupTest() {
	s.Start(&nilservice.Config{
		NShards:       s.ShardsNum,
		HttpUrl:       rpc.GetSockPath(s.T()),
		ZeroStateYaml: s.zerostateCfg,
		RunMode:       nilservice.CollatorsOnlyRunMode,
	})
}

func (s *SuiteMultiCurrencyRpc) TearDownTest() {
	s.Cancel()
}

// This test seems to quite big and complex, but there is no obvious way how to split it.
func (s *SuiteMultiCurrencyRpc) TestMultiCurrency() { //nolint

	currency1 := CreateTokenId(&s.walletAddress1)
	currency2 := CreateTokenId(&s.walletAddress2)

	s.Run("Initialize currency", func() {
		data := s.AbiPack(s.abiWallet, "setCurrencyName", "token1")
		receipt := s.SendExternalMessageNoCheck(data, s.walletAddress1)
		s.Require().True(receipt.Success)

		data = s.AbiPack(s.abiWallet, "mintCurrency", big.NewInt(100))
		receipt = s.SendExternalMessageNoCheck(data, s.walletAddress1)
		s.Require().True(receipt.Success)

		s.Run("Check currency is initialized", func() {
			currencies, err := s.Client.GetCurrencies(s.walletAddress1, "latest")
			s.Require().NoError(err)
			s.Require().Len(currencies, 1)
			s.Equal(types.NewValueFromUint64(100), currencies[*currency1.id])
		})

		s.Run("Check currency name", func() {
			data := s.AbiPack(s.abiWallet, "getCurrencyName")
			data = s.CallGetter(s.walletAddress1, data, "latest", nil)
			nameRes := s.AbiUnpack(s.abiWallet, "getCurrencyName", data)
			name, ok := nameRes[0].(string)
			s.Require().True(ok)
			s.Require().Equal("token1", name)
		})

		s.Run("Check currency total supply", func() {
			data := s.AbiPack(s.abiWallet, "getCurrencyTotalSupply")
			data = s.CallGetter(s.walletAddress1, data, "latest", nil)
			results := s.AbiUnpack(s.abiWallet, "getCurrencyTotalSupply", data)
			totalSupply, ok := results[0].(*big.Int)
			s.Require().True(ok)
			s.Require().Equal(big.NewInt(100), totalSupply)
		})
	})

	checkManageCurrency := func(method string, arg int64, balance int64) {
		s.T().Helper()

		s.Run(method+" currency", func() {
			data, err := s.abiWallet.Pack(method+"Currency", big.NewInt(arg))
			s.Require().NoError(err)

			receipt := s.SendExternalMessageNoCheck(data, s.walletAddress1)
			s.Require().True(receipt.Success)

			s.Run(fmt.Sprintf("Check currency is %sed", method), func() {
				currencies, err := s.Client.GetCurrencies(s.walletAddress1, "latest")
				s.Require().NoError(err)
				s.Require().Len(currencies, 1)
				s.Equal(types.NewValueFromUint64(uint64(balance)), currencies[*currency1.id])
			})

			s.Run("Check currency total supply", func() {
				data := s.AbiPack(s.abiWallet, "getCurrencyTotalSupply")
				data = s.CallGetter(s.walletAddress1, data, "latest", nil)
				results := s.AbiUnpack(s.abiWallet, "getCurrencyTotalSupply", data)
				totalSupply, ok := results[0].(*big.Int)
				s.Require().True(ok)
				s.Require().Equal(big.NewInt(balance), totalSupply)
			})
		})
	}

	checkManageCurrency("mint", 350, 450)

	checkManageCurrency("burn", 100, 350)

	s.Run("Transfer currency via sendCurrency", func() {
		data := s.AbiPack(s.abiWallet, "sendCurrency", s.walletAddress2, *currency1.id, big.NewInt(100))

		receipt := s.SendExternalMessage(data, s.walletAddress1)
		s.Require().True(receipt.Success)
		s.Require().True(receipt.OutReceipts[0].Success)

		s.Run("Check currency is transferred", func() {
			currencies, err := s.Client.GetCurrencies(s.walletAddress1, "latest")
			s.Require().NoError(err)
			s.Require().Len(currencies, 1)
			s.Equal(types.NewValueFromUint64(250), currencies[*currency1.id])

			currencies, err = s.Client.GetCurrencies(s.walletAddress2, "latest")
			s.Require().NoError(err)
			s.Require().Len(currencies, 1)
			s.Equal(types.NewValueFromUint64(100), currencies[*currency1.id])
		})
	})

	s.Run("Send from Wallet1 to Wallet2 via asyncCall", func() {
		receipt := s.SendMessageViaWalletNoCheck(s.walletAddress1, s.walletAddress2, execution.MainPrivateKey, nil,
			types.Value{}, types.Value{},
			[]types.CurrencyBalance{{Currency: *currency1.id, Balance: types.NewValueFromUint64(50)}})
		s.Require().True(receipt.Success)
		s.Require().True(receipt.OutReceipts[0].Success)

		s.Run("Check currency is transferred", func() {
			currencies, err := s.Client.GetCurrencies(s.walletAddress1, "latest")
			s.Require().NoError(err)
			s.Require().Len(currencies, 1)
			s.Equal(types.NewValueFromUint64(200), currencies[*currency1.id])

			currencies, err = s.Client.GetCurrencies(s.walletAddress2, "latest")
			s.Require().NoError(err)
			s.Require().Len(currencies, 1)
			s.Equal(types.NewValueFromUint64(150), currencies[*currency1.id])

			// Cross-shard `Nil.currencyBalance` should fail
			s.Require().NotEqual(s.testAddress1_0.ShardId(), s.walletAddress2.ShardId())
			data := s.AbiPack(s.abiTest, "checkTokenBalance", s.walletAddress2, currency1.id, big.NewInt(150))
			receipt = s.SendExternalMessageNoCheck(data, s.testAddress1_0)
			s.Require().False(receipt.Success)
		})
	})

	var amount types.Value
	s.Require().NoError(amount.Set("1000000000000000000000"))

	s.Run("Create 2-nd currency from Wallet2", func() {
		data := s.AbiPack(s.abiWallet, "setCurrencyName", "token2")
		receipt := s.SendExternalMessageNoCheck(data, s.walletAddress2)
		s.Require().True(receipt.Success)

		data = s.AbiPack(s.abiWallet, "mintCurrency", amount.ToBig())
		receipt = s.SendExternalMessageNoCheck(data, s.walletAddress2)
		s.Require().True(receipt.Success)

		s.Run("Check currency and balance", func() {
			currencies, err := s.Client.GetCurrencies(s.walletAddress2, "latest")
			s.Require().NoError(err)
			s.Require().Len(currencies, 2)
			s.Equal(types.NewValueFromUint64(150), currencies[*currency1.id])
			s.Equal(amount, currencies[*currency2.id])
		})
	})

	s.Run("Send 1-st and 2-nd currencies from Wallet2 to Wallet3 (same shard)", func() {
		s.Require().Equal(s.walletAddress2.ShardId(), s.walletAddress3.ShardId())
		receipt := s.SendMessageViaWalletNoCheck(s.walletAddress2, s.walletAddress3, execution.MainPrivateKey, nil,
			types.Value{}, types.Value{},
			[]types.CurrencyBalance{
				{Currency: *currency1.id, Balance: types.NewValueFromUint64(10)},
				{Currency: *currency2.id, Balance: types.NewValueFromUint64(500)},
			})
		s.Require().True(receipt.Success)
		s.Require().True(receipt.OutReceipts[0].Success)

		s.Run("Check currencies are transferred", func() {
			currencies, err := s.Client.GetCurrencies(s.walletAddress3, "latest")
			s.Require().NoError(err)
			s.Require().Len(currencies, 2)
			s.Equal(types.NewValueFromUint64(10), currencies[*currency1.id])
			s.Equal(types.NewValueFromUint64(500), currencies[*currency2.id])

			currencies, err = s.Client.GetCurrencies(s.walletAddress2, "latest")
			s.Require().NoError(err)
			s.Require().Len(currencies, 2)
			s.Equal(types.NewValueFromUint64(140), currencies[*currency1.id])
			s.Equal(amount.Sub(types.NewValueFromUint64(500)), currencies[*currency2.id])
		})
	})

	s.Run("Fail to send insufficient amount of 1st currency", func() {
		receipt := s.SendMessageViaWalletNoCheck(s.walletAddress2, s.walletAddress3, execution.MainPrivateKey, nil,
			types.NewValueFromUint64(100_000), types.Value{},
			[]types.CurrencyBalance{{Currency: *currency1.id, Balance: types.NewValueFromUint64(700)}})
		s.Require().False(receipt.Success)
		s.Require().Contains(receipt.ErrorMessage, vm.ErrInsufficientBalance.Error())

		s.Run("Check currency is not sent", func() {
			currencies, err := s.Client.GetCurrencies(s.walletAddress2, "latest")
			s.Require().NoError(err)
			s.Require().Len(currencies, 2)
			s.Equal(types.NewValueFromUint64(140), currencies[*currency1.id])

			currencies, err = s.Client.GetCurrencies(s.walletAddress3, "latest")
			s.Require().NoError(err)
			s.Require().Len(currencies, 2)
			s.Equal(types.NewValueFromUint64(10), currencies[*currency1.id])
		})
	})

	///////////////////////////////////////////////////////////////////////////
	// Second part of testing: tests through TokensTest.sol

	currencyTest1 := CreateTokenId(&s.testAddress1_0)
	currencyTest2 := CreateTokenId(&s.testAddress1_1)

	s.Run("Create tokens for test addresses", func() {
		s.createCurrencyForTestContract(currencyTest1, types.NewValueFromUint64(1_000_000), "testToken1")
		s.createCurrencyForTestContract(currencyTest2, types.NewValueFromUint64(2_000_000), "testToken2")
	})

	s.Run("Call testCallWithTokensSync of testAddress1_0", func() {
		data, err := s.abiTest.Pack("testCallWithTokensSync", s.testAddress1_1,
			[]types.CurrencyBalance{{Currency: *currencyTest1.id, Balance: types.NewValueFromUint64(5000)}})
		s.Require().NoError(err)

		hash, err := s.Client.SendExternalMessage(data, s.testAddress1_0, nil, s.GasToValue(100_000))
		s.Require().NoError(err)
		receipt := s.WaitForReceipt(hash)
		s.Require().True(receipt.Success)

		s.Run("Check currency is debited from testAddress1_0", func() {
			currencies, err := s.Client.GetCurrencies(s.testAddress1_0, "latest")
			s.Require().NoError(err)
			s.Equal(types.NewValueFromUint64(1_000_000-5000), currencies[*currencyTest1.id])

			// Check balance via `Nil.currencyBalance` Solidity method
			data, err := s.abiTest.Pack("checkTokenBalance", types.EmptyAddress, currencyTest1.id, big.NewInt(1_000_000-5000))
			s.Require().NoError(err)
			receipt := s.SendExternalMessageNoCheck(data, s.testAddress1_0)
			s.Require().True(receipt.Success)
		})

		s.Run("Check currency is credited to testAddress1_1", func() {
			currencies, err := s.Client.GetCurrencies(s.testAddress1_1, "latest")
			s.Require().NoError(err)
			s.Equal(types.NewValueFromUint64(5000), currencies[*currencyTest1.id])
		})
	})

	invalidId := types.CurrencyId(types.HexToAddress("0x1234"))

	s.Run("Try to call with non-existent currency", func() {
		data, err := s.abiTest.Pack("testCallWithTokensSync", s.testAddress1_1,
			[]types.CurrencyBalance{
				{Currency: *currencyTest1.id, Balance: types.NewValueFromUint64(5000)},
				{Currency: invalidId, Balance: types.NewValueFromUint64(1)},
			})
		s.Require().NoError(err)

		hash, err := s.Client.SendExternalMessage(data, s.testAddress1_0, nil, s.GasToValue(100_000))
		s.Require().NoError(err)
		receipt := s.WaitForReceipt(hash)
		s.Require().False(receipt.Success)

		s.Run("Check currency of testAddress1_0", func() {
			currencies, err := s.Client.GetCurrencies(s.testAddress1_0, "latest")
			s.Require().NoError(err)
			s.Equal(types.NewValueFromUint64(1_000_000-5000), currencies[*currencyTest1.id])
		})

		s.Run("Check currency of testAddress1_1", func() {
			currencies, err := s.Client.GetCurrencies(s.testAddress1_1, "latest")
			s.Require().NoError(err)
			s.Equal(types.NewValueFromUint64(5000), currencies[*currencyTest1.id])
		})
	})

	s.Run("Call testCallWithTokensAsync of testAddress1_0", func() {
		data, err := s.abiTest.Pack("testCallWithTokensAsync", s.testAddress1_1,
			[]types.CurrencyBalance{{Currency: *currencyTest1.id, Balance: types.NewValueFromUint64(5000)}})
		s.Require().NoError(err)

		hash, err := s.Client.SendExternalMessage(data, s.testAddress1_0, nil, s.GasToValue(100_000))
		s.Require().NoError(err)
		receipt := s.WaitForReceipt(hash)
		s.Require().True(receipt.Success)
		s.Require().Len(receipt.OutReceipts, 1)
		s.Require().True(receipt.OutReceipts[0].Success)

		s.Run("Check currency is debited from testAddress1_0", func() {
			currencies, err := s.Client.GetCurrencies(s.testAddress1_0, "latest")
			s.Require().NoError(err)
			s.Equal(types.NewValueFromUint64(1_000_000-5000-5000), currencies[*currencyTest1.id])
		})

		s.Run("Check currency is credited to testAddress1_1", func() {
			currencies, err := s.Client.GetCurrencies(s.testAddress1_1, "latest")
			s.Require().NoError(err)
			s.Equal(types.NewValueFromUint64(5000+5000), currencies[*currencyTest1.id])
		})
	})

	s.Run("Try to call with non-existent currency", func() {
		data, err := s.abiTest.Pack("testCallWithTokensAsync", s.testAddress1_1,
			[]types.CurrencyBalance{
				{Currency: *currencyTest1.id, Balance: types.NewValueFromUint64(5000)},
				{Currency: invalidId, Balance: types.NewValueFromUint64(1)},
			})
		s.Require().NoError(err)

		hash, err := s.Client.SendExternalMessage(data, s.testAddress1_0, nil, s.GasToValue(100_000))
		s.Require().NoError(err)
		receipt := s.WaitForReceipt(hash)
		s.Require().False(receipt.Success)
		s.Require().Empty(receipt.OutReceipts)

		s.Run("Check currency of testAddress1_0", func() {
			currencies, err := s.Client.GetCurrencies(s.testAddress1_0, "latest")
			s.Require().NoError(err)
			s.Equal(types.NewValueFromUint64(1_000_000-5000-5000), currencies[*currencyTest1.id])
		})

		s.Run("Check currency of testAddress1_1", func() {
			currencies, err := s.Client.GetCurrencies(s.testAddress1_1, "latest")
			s.Require().NoError(err)
			s.Equal(types.NewValueFromUint64(5000+5000), currencies[*currencyTest1.id])
		})
	})

	amountTest1 := s.getCurrencyBalance(&s.testAddress1_0, currencyTest1)
	amountTest2 := s.getCurrencyBalance(&s.testAddress1_1, currencyTest1)

	s.Run("Call testSendTokensSync", func() {
		data, err := s.abiTest.Pack("testSendTokensSync", s.testAddress1_1, big.NewInt(5000), false)
		s.Require().NoError(err)

		hash, err := s.Client.SendExternalMessage(data, s.testAddress1_0, nil, s.GasToValue(100_000))
		s.Require().NoError(err)
		receipt := s.WaitForReceipt(hash)
		s.Require().True(receipt.Success)
		s.Require().Empty(receipt.OutReceipts)
		s.Require().Empty(receipt.OutMessages)

		s.Run("Check currency was debited from testAddress1_0", func() {
			currencies, err := s.Client.GetCurrencies(s.testAddress1_0, "latest")
			s.Require().NoError(err)
			s.Equal(amountTest1.Sub64(5000), currencies[*currencyTest1.id])
		})

		s.Run("Check currency was credited to testAddress1_1", func() {
			currencies, err := s.Client.GetCurrencies(s.testAddress1_1, "latest")
			s.Require().NoError(err)
			s.Equal(amountTest2.Add64(5000), currencies[*currencyTest1.id])
		})
	})

	s.Run("Call testSendTokensSync with fail flag", func() {
		data, err := s.abiTest.Pack("testSendTokensSync", s.testAddress1_1, big.NewInt(5000), true)
		s.Require().NoError(err)

		hash, err := s.Client.SendExternalMessage(data, s.testAddress1_0, nil, s.GasToValue(100_000))
		s.Require().NoError(err)
		receipt := s.WaitForReceipt(hash)
		s.Require().False(receipt.Success)

		s.Run("Check currency of testAddress1_0", func() {
			currencies, err := s.Client.GetCurrencies(s.testAddress1_0, "latest")
			s.Require().NoError(err)
			s.Equal(amountTest1.Sub64(5000), currencies[*currencyTest1.id])
		})

		s.Run("Check currency of testAddress1_1", func() {
			currencies, err := s.Client.GetCurrencies(s.testAddress1_1, "latest")
			s.Require().NoError(err)
			s.Equal(amountTest2.Add64(5000), currencies[*currencyTest1.id])
		})
	})

	///////////////////////////////////////////////////////////////////////////
	// Call `testSendTokensSync` for address in different shard - should fail
	s.Run("Fail call testSendTokensSync for address in different shard", func() {
		data, err := s.abiTest.Pack("testSendTokensSync", s.walletAddress3, big.NewInt(5000), false)
		s.Require().NoError(err)

		hash, err := s.Client.SendExternalMessage(data, s.testAddress1_0, nil, s.GasToValue(100_000))
		s.Require().NoError(err)
		receipt := s.WaitForReceipt(hash)
		s.Require().False(receipt.Success)

		s.Run("Check currency of testAddress1_0", func() {
			currencies, err := s.Client.GetCurrencies(s.testAddress1_0, "latest")
			s.Require().NoError(err)
			s.Require().Equal(amountTest1.Sub64(5000), currencies[*currencyTest1.id])
		})
	})
}

func (s *SuiteMultiCurrencyRpc) TestCurrencyViaCall() {
	// Check that it's possible to call some function via eth_call
	// that works with currencies without crashes/errors.

	data := s.AbiPack(s.abiWallet, "mintCurrency", big.NewInt(100))
	res, err := s.Client.Call(&jsonrpc.CallArgs{
		To:        s.walletAddress1,
		Data:      (*hexutil.Bytes)(&data),
		FeeCredit: types.NewValueFromUint64(1000000),
	}, "latest", nil)
	s.Require().NoError(err)
	s.Require().Empty(res.Error)
	s.Require().Positive(res.CoinsUsed.Uint64())
}

func (s *SuiteMultiCurrencyRpc) TestBounce() {
	var (
		data       []byte
		currencies types.CurrenciesMap
		receipt    *jsonrpc.RPCReceipt
		err        error
	)

	currencyWallet1 := CreateTokenId(&s.walletAddress1)

	s.createCurrencyForTestContract(currencyWallet1, types.NewValueFromUint64(1_000_000), "wallet1")

	data, err = s.abiTest.Pack("receiveTokens", true)
	s.Require().NoError(err)

	receipt = s.SendMessageViaWalletNoCheck(s.walletAddress1, s.testAddress1_0, execution.MainPrivateKey, data,
		types.NewZeroValue(), types.NewValueFromUint64(2_000_000),
		[]types.CurrencyBalance{{Currency: *currencyWallet1.id, Balance: types.NewValueFromUint64(100)}})
	s.Require().True(receipt.Success)
	s.Require().Len(receipt.OutReceipts, 1)
	s.Require().False(receipt.OutReceipts[0].Success)

	// Check that nothing credited tp destination account
	currencies, err = s.Client.GetCurrencies(s.testAddress1_0, "latest")
	s.Require().NoError(err)
	s.Require().Equal(types.NewValueFromUint64(0), currencies[*currencyWallet1.id])

	// Check that currency wasn't changed
	currencies, err = s.Client.GetCurrencies(s.walletAddress1, "latest")
	s.Require().NoError(err)
	s.Require().Equal(types.NewValueFromUint64(1_000_000), currencies[*currencyWallet1.id])
}

func (s *SuiteMultiCurrencyRpc) TestIncomingBalance() {
	var (
		data    []byte
		receipt *jsonrpc.RPCReceipt
		err     error
	)

	currencyWallet1 := CreateTokenId(&s.walletAddress1)

	checkBalance := func(msgTokens *big.Int, accTokens *big.Int, receipt *jsonrpc.RPCReceipt) {
		a, err := s.abiTest.Events["tokenMsgBalance"].Inputs.Unpack(receipt.Logs[0].Data)
		s.Require().NoError(err)
		res, ok := a[0].(*big.Int)
		s.Require().True(ok)
		s.Require().Equal(*msgTokens, *res)

		a, err = s.abiTest.Events["tokenBalance"].Inputs.Unpack(receipt.Logs[1].Data)
		s.Require().NoError(err)
		res, ok = a[0].(*big.Int)
		s.Require().True(ok)
		s.Require().Equal(*accTokens, *res)
	}

	s.createCurrencyForTestContract(currencyWallet1, types.NewValueFromUint64(1_000_000), "wallet1")

	data, err = s.abiTest.Pack("checkIncomingToken", *currencyWallet1.id)
	s.Require().NoError(err)

	receipt = s.SendMessageViaWalletNoCheck(s.walletAddress1, s.testAddress1_0, execution.MainPrivateKey, data,
		types.NewZeroValue(), types.NewValueFromUint64(2_000_000),
		[]types.CurrencyBalance{{Currency: *currencyWallet1.id, Balance: types.NewValueFromUint64(100)}})
	s.Require().True(receipt.AllSuccess())

	checkBalance(big.NewInt(100), big.NewInt(100), receipt.OutReceipts[0])

	receipt = s.SendMessageViaWalletNoCheck(s.walletAddress1, s.testAddress1_0, execution.MainPrivateKey, data,
		types.NewZeroValue(), types.NewValueFromUint64(2_000_000),
		[]types.CurrencyBalance{{Currency: *currencyWallet1.id, Balance: types.NewValueFromUint64(20_000)}})
	s.Require().True(receipt.AllSuccess())

	checkBalance(big.NewInt(20_000), big.NewInt(20_100), receipt.OutReceipts[0])
}

// NameTokensTestNoExternalAccess contract has no external access to currency
func (s *SuiteMultiCurrencyRpc) TestNoExternalAccess() {
	abiTest, err := contracts.GetAbi(contracts.NameTokensTestNoExternalAccess)
	s.Require().NoError(err)

	currency := CreateTokenId(&s.testAddressNoAccess)

	data := s.AbiPack(abiTest, "setCurrencyName", "TOKEN")
	receipt := s.SendExternalMessageNoCheck(data, *currency.address)
	s.Require().False(receipt.Success)
	s.Require().Equal("ExecutionReverted", receipt.Status)

	data = s.AbiPack(abiTest, "mintCurrency", big.NewInt(100_000))
	receipt = s.SendExternalMessageNoCheck(data, *currency.address)
	s.Require().False(receipt.Success)
	s.Require().Equal("ExecutionReverted", receipt.Status)

	data = s.AbiPack(abiTest, "sendCurrency", s.testAddress1_1, *currency.id, big.NewInt(100_000))
	receipt = s.SendExternalMessageNoCheck(data, *currency.address)
	s.Require().False(receipt.Success)
	s.Require().Equal("ExecutionReverted", receipt.Status)
}

func (s *SuiteMultiCurrencyRpc) getCurrencyBalance(address *types.Address, currency *CurrencyId) types.Value {
	s.T().Helper()

	currencies, err := s.Client.GetCurrencies(*address, "latest")
	s.Require().NoError(err)
	return currencies[*currency.id]
}

func (s *SuiteMultiCurrencyRpc) createCurrencyForTestContract(currency *CurrencyId, amount types.Value, name string) {
	s.T().Helper()

	data := s.AbiPack(s.abiTest, "setCurrencyName", name)
	receipt := s.SendExternalMessageNoCheck(data, *currency.address)
	s.Require().True(receipt.Success)

	data = s.AbiPack(s.abiTest, "mintCurrency", amount.ToBig())
	receipt = s.SendExternalMessageNoCheck(data, *currency.address)
	s.Require().True(receipt.Success)

	// Check currency is created and balance is correct
	currencies, err := s.Client.GetCurrencies(*currency.address, "latest")
	s.Require().NoError(err)
	s.Require().GreaterOrEqual(len(currencies), 1)
	s.Equal(amount, currencies[*currency.id])

	// Check via getOwnCurrencyBalance method
	data = s.AbiPack(s.abiTest, "getOwnCurrencyBalance")
	data = s.CallGetter(*currency.address, data, "latest", nil)
	results := s.AbiUnpack(s.abiTest, "getOwnCurrencyBalance", data)
	res, ok := results[0].(*big.Int)
	s.Require().True(ok)
	s.Require().Equal(amount.ToBig(), res)
}

type CurrencyId struct {
	address *types.Address
	id      *types.CurrencyId
}

func CreateTokenId(address *types.Address) *CurrencyId {
	id := types.CurrencyId(*address)
	return &CurrencyId{
		address: address,
		id:      &id,
	}
}

func TestMultiCurrency(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteMultiCurrencyRpc))
}
