package rpctest

import (
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/NilFoundation/nil/nil/internal/collate"
	"github.com/NilFoundation/nil/nil/internal/contracts"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/nilservice"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/suite"
)

type SuiteMultiCurrencyRpc struct {
	RpcSuite
	walletAddress1 types.Address
	walletAddress2 types.Address
	walletAddress3 types.Address
	testAddress1_0 types.Address
	testAddress1_1 types.Address
	abiMinter      *abi.ABI
	abiTest        *abi.ABI
	abiWallet      *abi.ABI
	zerostateCfg   string
}

func (s *SuiteMultiCurrencyRpc) SetupSuite() {
	s.shardsNum = 4

	s.walletAddress1 = contracts.WalletAddress(s.T(), 2, []byte{0}, execution.MainPublicKey)
	s.walletAddress2 = contracts.WalletAddress(s.T(), 3, []byte{1}, execution.MainPublicKey)
	s.walletAddress3 = contracts.WalletAddress(s.T(), 3, []byte{3}, execution.MainPublicKey)

	var err error
	s.testAddress1_0, err = contracts.CalculateAddress(contracts.NameTokensTest, types.MinterAddress.ShardId(), []byte{1})
	s.Require().NoError(err)

	s.testAddress1_1, err = contracts.CalculateAddress(contracts.NameTokensTest, types.MinterAddress.ShardId(), []byte{2})
	s.Require().NoError(err)

	zerostateTmpl := `
contracts:
- name: Minter
  address: {{ .MinterAddress }}
  value: 100000000000000
  contract: Minter
  ctorArgs: [{{ .MainPublicKey }}]
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
`
	s.zerostateCfg, err = common.ParseTemplate(zerostateTmpl, map[string]interface{}{
		"MinterAddress":        types.MinterAddress.Hex(),
		"MainPublicKey":        hexutil.Encode(execution.MainPublicKey),
		"TestAddress1":         s.walletAddress1.Hex(),
		"TestAddress2":         s.walletAddress2.Hex(),
		"TestAddress3":         s.walletAddress3.Hex(),
		"TokensTestAddress1_0": s.testAddress1_0.Hex(),
		"TokensTestAddress1_1": s.testAddress1_1.Hex(),
	})
	s.Require().NoError(err)

	s.abiMinter, err = contracts.GetAbi(contracts.NameMinter)
	s.Require().NoError(err)

	s.abiWallet, err = contracts.GetAbi("Wallet")
	s.Require().NoError(err)

	s.abiTest, err = contracts.GetAbi(contracts.NameTokensTest)
	s.Require().NoError(err)
}

func (s *SuiteMultiCurrencyRpc) SetupTest() {
	s.start(&nilservice.Config{
		NShards:              s.shardsNum,
		HttpPort:             8534,
		Topology:             collate.TrivialShardTopologyId,
		ZeroState:            s.zerostateCfg,
		CollatorTickPeriodMs: 100,
		GasBasePrice:         10,
		RunMode:              nilservice.CollatorsOnlyRunMode,
	})
}

func (s *SuiteMultiCurrencyRpc) TearDownTest() {
	s.cancel()
}

// This test seems to quite big and complex, but there is no obvious way how to split it.
func (s *SuiteMultiCurrencyRpc) TestMultiCurrency() { //nolint
	multiCurrAbi, err := contracts.GetAbi(contracts.NameNilCurrencyBase)
	s.Require().NoError(err)

	currency1 := CreateTokenId(&s.walletAddress1)
	currency2 := CreateTokenId(&s.walletAddress2)

	s.Run("Create currency", func() {
		data, err := multiCurrAbi.Pack("createToken", big.NewInt(100), "token1", false)
		s.Require().NoError(err)

		receipt := s.sendExternalMessage(data, s.walletAddress1)
		s.Require().True(receipt.Success)
		s.Require().True(receipt.OutReceipts[0].Success)

		s.Run("Check currency is created", func() {
			currencies, err := s.client.GetCurrencies(types.MinterAddress, "latest")
			s.Require().NoError(err)
			s.Require().Len(currencies, 1)
			s.Equal(types.NewValueFromUint64(100), currencies[currency1.idStr])
		})

		s.Run("Check currency name", func() {
			data, err := s.abiMinter.Pack("getName", &currency1.idInt)
			s.Require().NoError(err)
			data = s.CallGetter(types.MinterAddress, data, "latest")
			nameRes, err := s.abiMinter.Unpack("getName", data)
			s.Require().NoError(err)
			name, ok := nameRes[0].(string)
			s.Require().True(ok)
			s.Require().Equal("token1", name)
		})
	})

	s.Run("Mint currency", func() {
		data, err := multiCurrAbi.Pack("mintToken", big.NewInt(250), false)
		s.Require().NoError(err)

		receipt := s.sendExternalMessage(data, s.walletAddress1)
		s.Require().True(receipt.Success)
		s.Require().True(receipt.OutReceipts[0].Success)

		s.Run("Check currency is minted", func() {
			currencies, err := s.client.GetCurrencies(types.MinterAddress, "latest")
			s.Require().NoError(err)
			s.Require().Len(currencies, 1)
			s.Equal(types.NewValueFromUint64(350), currencies[currency1.idStr])
		})
	})

	s.Run("Transfer currency", func() {
		data, err := multiCurrAbi.Pack("withdrawToken", big.NewInt(100), s.walletAddress1)
		s.Require().NoError(err)

		receipt := s.sendExternalMessage(data, s.walletAddress1)
		s.Require().True(receipt.Success)
		s.Require().True(receipt.OutReceipts[0].Success)

		for !receipt.IsComplete() {
			time.Sleep(100 * time.Millisecond)
		}

		fmt.Printf("receipt: %v\n", receipt)

		s.Run("Check currency is transferred", func() {
			currencies, err := s.client.GetCurrencies(types.MinterAddress, "latest")
			s.Require().NoError(err)
			s.Require().Len(currencies, 1)
			s.Equal(types.NewValueFromUint64(250), currencies[currency1.idStr])

			currencies, err = s.client.GetCurrencies(s.walletAddress1, "latest")
			s.Require().NoError(err)
			s.Require().Len(currencies, 1)
			s.Equal(types.NewValueFromUint64(100), currencies[currency1.idStr])
		})
	})

	s.Run("Send from Wallet1 to Wallet2", func() {
		receipt := s.sendMessageViaWalletNoCheck(s.walletAddress1, s.walletAddress2, execution.MainPrivateKey, nil,
			s.gasToValue(100_000), types.NewValueFromUint64(2_000_000),
			[]types.CurrencyBalance{{Currency: *currency1.id, Balance: types.NewValueFromUint64(40)}})
		s.Require().True(receipt.Success)
		s.Require().True(receipt.OutReceipts[0].Success)

		s.Run("Check currency is transferred", func() {
			currencies, err := s.client.GetCurrencies(s.walletAddress1, "latest")
			s.Require().NoError(err)
			s.Require().Len(currencies, 1)
			s.Equal(types.NewValueFromUint64(60), currencies[currency1.idStr])

			currencies, err = s.client.GetCurrencies(s.walletAddress2, "latest")
			s.Require().NoError(err)
			s.Require().Len(currencies, 1)
			s.Equal(types.NewValueFromUint64(40), currencies[currency1.idStr])
		})
	})

	s.Run("Fail to create same currency from different address", func() {
		data, err := s.abiMinter.Pack("create", big.NewInt(100), s.walletAddress1, "token1", types.EmptyAddress)
		s.Require().NoError(err)

		receipt := s.sendMessageViaWallet(s.walletAddress2, types.MinterAddress, execution.MainPrivateKey, data, types.Value{})
		s.Require().True(receipt.Success)
		s.Require().False(receipt.OutReceipts[0].Success)
	})

	var amount types.Value
	s.Require().NoError(amount.Set("1000000000000000000000"))

	s.Run("Create 2-nd currency from Wallet2", func() {
		data, err := s.abiMinter.Pack("create", amount.ToBig(), s.walletAddress2, "token2", types.EmptyAddress)
		s.Require().NoError(err)

		receipt := s.sendMessageViaWallet(s.walletAddress2, types.MinterAddress, execution.MainPrivateKey, data, types.Value{})
		s.Require().True(receipt.Success)
		s.Require().True(receipt.OutReceipts[0].Success)

		s.Run("Check currency and balance", func() {
			currencies, err := s.client.GetCurrencies(types.MinterAddress, "latest")
			s.Require().NoError(err)
			s.Require().Len(currencies, 2)
			s.Equal(types.NewValueFromUint64(250), currencies[currency1.idStr])
			s.Equal(amount, currencies[currency2.idStr])
		})
	})

	s.Run("Transfer all 2-nd currency to Wallet2", func() {
		data, err := s.abiMinter.Pack("withdraw", &currency2.idInt, amount.ToBig(), s.walletAddress2)
		s.Require().NoError(err)

		receipt := s.sendMessageViaWallet(s.walletAddress2, types.MinterAddress, execution.MainPrivateKey, data, types.NewValueFromUint64(968650))
		s.Require().True(receipt.Success)
		s.Require().True(receipt.OutReceipts[0].Success)

		s.Run("Check currency is transferred", func() {
			currencies, err := s.client.GetCurrencies(types.MinterAddress, "latest")
			s.Require().NoError(err)
			s.Require().Len(currencies, 2)
			s.Equal(types.NewValueFromUint64(250), currencies[currency1.idStr])
			s.Equal(types.NewValueFromUint64(0), currencies[currency2.idStr])

			currencies, err = s.client.GetCurrencies(s.walletAddress2, "latest")
			s.Require().NoError(err)
			s.Require().Len(currencies, 2)
			s.Equal(types.NewValueFromUint64(40), currencies[currency1.idStr])
			s.Equal(amount, currencies[currency2.idStr])
		})
	})

	s.Run("Send 1-st and 2-nd currencies Wallet2 to Wallet3 (same shard)", func() {
		s.Require().Equal(s.walletAddress2.ShardId(), s.walletAddress3.ShardId())
		receipt := s.sendMessageViaWalletNoCheck(s.walletAddress2, s.walletAddress3, execution.MainPrivateKey, nil,
			s.gasToValue(1_000_000), types.NewValueFromUint64(2_000_000),
			[]types.CurrencyBalance{
				{Currency: *currency1.id, Balance: types.NewValueFromUint64(10)},
				{Currency: *currency2.id, Balance: types.NewValueFromUint64(500)},
			})
		s.Require().True(receipt.Success)
		s.Require().True(receipt.OutReceipts[0].Success)

		s.Run("Check currencies are transferred", func() {
			currencies, err := s.client.GetCurrencies(s.walletAddress3, "latest")
			s.Require().NoError(err)
			s.Require().Len(currencies, 2)
			s.Equal(types.NewValueFromUint64(10), currencies[currency1.idStr])
			s.Equal(types.NewValueFromUint64(500), currencies[currency2.idStr])

			currencies, err = s.client.GetCurrencies(s.walletAddress2, "latest")
			s.Require().NoError(err)
			s.Require().Len(currencies, 2)
			s.Equal(types.NewValueFromUint64(30), currencies[currency1.idStr])
			s.Equal(amount.Sub(types.NewValueFromUint64(500)), currencies[currency2.idStr])
		})
	})

	s.Run("Fail to transfer 1st currency to Wallet2", func() {
		data, err := s.abiMinter.Pack("withdraw", &currency1.idInt, big.NewInt(2), s.walletAddress2)
		s.Require().NoError(err)

		receipt := s.sendMessageViaWallet(s.walletAddress2, types.MinterAddress, execution.MainPrivateKey, data, types.Value{})
		s.Require().True(receipt.Success)
		s.Require().False(receipt.OutReceipts[0].Success)
	})

	s.Run("Fail to send insufficient amount of 1st currency", func() {
		receipt := s.sendMessageViaWalletNoCheck(s.walletAddress2, s.walletAddress3, execution.MainPrivateKey, nil,
			s.gasToValue(1_000_000), types.NewValueFromUint64(2_000_000),
			[]types.CurrencyBalance{{Currency: *currency1.id, Balance: types.NewValueFromUint64(700)}})
		s.Require().False(receipt.Success)

		s.Run("Check currency is not sent", func() {
			currencies, err := s.client.GetCurrencies(s.walletAddress2, "latest")
			s.Require().NoError(err)
			s.Require().Len(currencies, 2)
			s.Equal(types.NewValueFromUint64(30), currencies[currency1.idStr])

			currencies, err = s.client.GetCurrencies(s.walletAddress3, "latest")
			s.Require().NoError(err)
			s.Require().Len(currencies, 2)
			s.Equal(types.NewValueFromUint64(10), currencies[currency1.idStr])
		})
	})

	s.Run("Mint and transfer currency to Wallet3", func() {
		currencies, err := s.client.GetCurrencies(types.MinterAddress, "latest")
		s.Require().NoError(err)
		minterCurrency1 := currencies[currency1.idStr]
		currencies, err = s.client.GetCurrencies(s.walletAddress1, "latest")
		s.Require().NoError(err)
		walletCurrency1 := currencies[currency1.idStr]

		data, err := s.abiMinter.Pack("mint", &currency1.idInt, big.NewInt(1000), s.walletAddress3)
		s.Require().NoError(err)

		receipt := s.sendMessageViaWallet(s.walletAddress1, types.MinterAddress, execution.MainPrivateKey, data, types.Value{})
		s.Require().True(receipt.Success)
		s.Require().True(receipt.OutReceipts[0].Success)

		s.Run("Check currency is minted", func() {
			currencies, err := s.client.GetCurrencies(types.MinterAddress, "latest")
			s.Require().NoError(err)
			s.Equal(minterCurrency1, currencies[currency1.idStr])
		})

		s.Run("Check currency of wallet1", func() {
			currencies, err := s.client.GetCurrencies(s.walletAddress1, "latest")
			s.Require().NoError(err)
			s.Equal(walletCurrency1, currencies[currency1.idStr])
		})

		s.Run("Check currency of wallet3", func() {
			currencies, err := s.client.GetCurrencies(s.walletAddress3, "latest")
			s.Require().NoError(err)
			s.Equal(types.NewValueFromUint64(1010), currencies[currency1.idStr])
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
			[]types.CurrencyBalanceAbiCompatible{{Currency: currencyTest1.idInt, Balance: uint256.NewInt(5000).ToBig()}})
		s.Require().NoError(err)

		hash, err := s.client.SendExternalMessage(data, s.testAddress1_0, nil)
		s.Require().NoError(err)
		receipt := s.waitForReceipt(s.testAddress1_0.ShardId(), hash)
		s.Require().True(receipt.Success)

		s.Run("Check currency is debited from testAddress1_0", func() {
			currencies, err := s.client.GetCurrencies(s.testAddress1_0, "latest")
			s.Require().NoError(err)
			s.Equal(types.NewValueFromUint64(1_000_000-5000), currencies[currencyTest1.idStr])
		})

		s.Run("Check currency is credited to testAddress1_1", func() {
			currencies, err := s.client.GetCurrencies(s.testAddress1_1, "latest")
			s.Require().NoError(err)
			s.Equal(types.NewValueFromUint64(5000), currencies[currencyTest1.idStr])
		})
	})

	s.Run("Try to call with non-existent currency", func() {
		data, err := s.abiTest.Pack("testCallWithTokensSync", s.testAddress1_1,
			[]types.CurrencyBalanceAbiCompatible{
				{Currency: currencyTest1.idInt, Balance: uint256.NewInt(5000).ToBig()},
				{Currency: big.NewInt(0).Add(currencyTest1.idInt, big.NewInt(1)), Balance: uint256.NewInt(1).ToBig()},
			})
		s.Require().NoError(err)

		hash, err := s.client.SendExternalMessage(data, s.testAddress1_0, nil)
		s.Require().NoError(err)
		receipt := s.waitForReceipt(s.testAddress1_0.ShardId(), hash)
		s.Require().False(receipt.Success)

		s.Run("Check currency of testAddress1_0", func() {
			currencies, err := s.client.GetCurrencies(s.testAddress1_0, "latest")
			s.Require().NoError(err)
			s.Equal(types.NewValueFromUint64(1_000_000-5000), currencies[currencyTest1.idStr])
		})

		s.Run("Check currency of testAddress1_1", func() {
			currencies, err := s.client.GetCurrencies(s.testAddress1_1, "latest")
			s.Require().NoError(err)
			s.Equal(types.NewValueFromUint64(5000), currencies[currencyTest1.idStr])
		})
	})

	s.Run("Call testCallWithTokensAsync of testAddress1_0", func() {
		data, err := s.abiTest.Pack("testCallWithTokensAsync", s.testAddress1_1,
			[]types.CurrencyBalanceAbiCompatible{{Currency: currencyTest1.idInt, Balance: uint256.NewInt(5000).ToBig()}})
		s.Require().NoError(err)

		hash, err := s.client.SendExternalMessage(data, s.testAddress1_0, nil)
		s.Require().NoError(err)
		receipt := s.waitForReceipt(s.testAddress1_0.ShardId(), hash)
		s.Require().True(receipt.Success)
		s.Require().Len(receipt.OutReceipts, 1)
		s.Require().True(receipt.OutReceipts[0].Success)

		s.Run("Check currency is debited from testAddress1_0", func() {
			currencies, err := s.client.GetCurrencies(s.testAddress1_0, "latest")
			s.Require().NoError(err)
			s.Equal(types.NewValueFromUint64(1_000_000-5000-5000), currencies[currencyTest1.idStr])
		})

		s.Run("Check currency is credited to testAddress1_1", func() {
			currencies, err := s.client.GetCurrencies(s.testAddress1_1, "latest")
			s.Require().NoError(err)
			s.Equal(types.NewValueFromUint64(5000+5000), currencies[currencyTest1.idStr])
		})
	})

	s.Run("Try to call with non-existent currency", func() {
		data, err := s.abiTest.Pack("testCallWithTokensAsync", s.testAddress1_1,
			[]types.CurrencyBalanceAbiCompatible{
				{Currency: currencyTest1.idInt, Balance: uint256.NewInt(5000).ToBig()},
				{Currency: big.NewInt(0).Add(currencyTest1.idInt, big.NewInt(1)), Balance: uint256.NewInt(1).ToBig()},
			})
		s.Require().NoError(err)

		hash, err := s.client.SendExternalMessage(data, s.testAddress1_0, nil)
		s.Require().NoError(err)
		receipt := s.waitForReceipt(s.testAddress1_0.ShardId(), hash)
		s.Require().False(receipt.Success)
		s.Require().Empty(receipt.OutReceipts)

		s.Run("Check currency of testAddress1_0", func() {
			currencies, err := s.client.GetCurrencies(s.testAddress1_0, "latest")
			s.Require().NoError(err)
			s.Equal(types.NewValueFromUint64(1_000_000-5000-5000), currencies[currencyTest1.idStr])
		})

		s.Run("Check currency of testAddress1_1", func() {
			currencies, err := s.client.GetCurrencies(s.testAddress1_1, "latest")
			s.Require().NoError(err)
			s.Equal(types.NewValueFromUint64(5000+5000), currencies[currencyTest1.idStr])
		})
	})

	amountTest1 := s.getCurrencyBalance(&s.testAddress1_0, currencyTest1)
	amountTest2 := s.getCurrencyBalance(&s.testAddress1_1, currencyTest1)

	s.Run("Call testSendTokensSync", func() {
		data, err := s.abiTest.Pack("testSendTokensSync", s.testAddress1_1, big.NewInt(5000), false)
		s.Require().NoError(err)

		hash, err := s.client.SendExternalMessage(data, s.testAddress1_0, nil)
		s.Require().NoError(err)
		receipt := s.waitForReceipt(s.testAddress1_0.ShardId(), hash)
		s.Require().True(receipt.Success)
		s.Require().Empty(receipt.OutReceipts)
		s.Require().Empty(receipt.OutMessages)

		s.Run("Check currency was debited from testAddress1_0", func() {
			currencies, err := s.client.GetCurrencies(s.testAddress1_0, "latest")
			s.Require().NoError(err)
			s.Equal(amountTest1.Sub64(5000), currencies[currencyTest1.idStr])
		})

		s.Run("Check currency was credited to testAddress1_1", func() {
			currencies, err := s.client.GetCurrencies(s.testAddress1_1, "latest")
			s.Require().NoError(err)
			s.Equal(amountTest2.Add64(5000), currencies[currencyTest1.idStr])
		})
	})

	s.Run("Call testSendTokensSync with fail flag", func() {
		data, err := s.abiTest.Pack("testSendTokensSync", s.testAddress1_1, big.NewInt(5000), true)
		s.Require().NoError(err)

		hash, err := s.client.SendExternalMessage(data, s.testAddress1_0, nil)
		s.Require().NoError(err)
		receipt := s.waitForReceipt(s.testAddress1_0.ShardId(), hash)
		s.Require().False(receipt.Success)

		s.Run("Check currency of testAddress1_0", func() {
			currencies, err := s.client.GetCurrencies(s.testAddress1_0, "latest")
			s.Require().NoError(err)
			s.Equal(amountTest1.Sub64(5000), currencies[currencyTest1.idStr])
		})

		s.Run("Check currency of testAddress1_1", func() {
			currencies, err := s.client.GetCurrencies(s.testAddress1_1, "latest")
			s.Require().NoError(err)
			s.Equal(amountTest2.Add64(5000), currencies[currencyTest1.idStr])
		})
	})

	///////////////////////////////////////////////////////////////////////////
	// Call `testSendTokensSync` for address in different shard - should fail
	s.Run("Fail call testSendTokensSync for address in different shard", func() {
		data, err := s.abiTest.Pack("testSendTokensSync", s.walletAddress3, big.NewInt(5000), false)
		s.Require().NoError(err)

		hash, err := s.client.SendExternalMessage(data, s.testAddress1_0, nil)
		s.Require().NoError(err)
		receipt := s.waitForReceipt(s.testAddress1_0.ShardId(), hash)
		s.Require().False(receipt.Success)

		s.Run("Check currency of testAddress1_0", func() {
			currencies, err := s.client.GetCurrencies(s.testAddress1_0, "latest")
			s.Require().NoError(err)
			s.Require().Equal(amountTest1.Sub64(5000), currencies[currencyTest1.idStr])
		})
	})
}

func (s *SuiteMultiCurrencyRpc) TestBounce() {
	var (
		data       []byte
		currencies types.CurrenciesMap
		receipt    *jsonrpc.RPCReceipt
		err        error
	)

	currencyWallet1 := CreateTokenId(&s.walletAddress1)

	s.createCurrencyForWallet(currencyWallet1, big.NewInt(1_000_000), "wallet1", true)

	data, err = s.abiTest.Pack("receiveTokens", true)
	s.Require().NoError(err)

	receipt = s.sendMessageViaWalletNoCheck(s.walletAddress1, s.testAddress1_0, execution.MainPrivateKey, data,
		s.gasToValue(100_000), types.NewValueFromUint64(2_000_000),
		[]types.CurrencyBalance{{Currency: *currencyWallet1.id, Balance: types.NewValueFromUint64(100)}})
	s.Require().True(receipt.Success)
	s.Require().Len(receipt.OutReceipts, 1)
	s.Require().False(receipt.OutReceipts[0].Success)

	// Check that nothing credited tp destination account
	currencies, err = s.client.GetCurrencies(s.testAddress1_0, "latest")
	s.Require().NoError(err)
	s.Require().Equal(types.NewValueFromUint64(0), currencies[currencyWallet1.idStr])

	// Check that currency wasn't changed
	currencies, err = s.client.GetCurrencies(s.walletAddress1, "latest")
	s.Require().NoError(err)
	s.Require().Equal(types.NewValueFromUint64(1_000_000), currencies[currencyWallet1.idStr])
}

func (s *SuiteMultiCurrencyRpc) getCurrencyBalance(address *types.Address, currency *CurrencyId) types.Value {
	s.T().Helper()

	currencies, err := s.client.GetCurrencies(*address, "latest")
	s.Require().NoError(err)
	return currencies[currency.idStr]
}

func (s *SuiteMultiCurrencyRpc) TestInfoAndShardId() {
	var (
		data []byte
		err  error
	)
	currencyWallet1 := CreateTokenId(&s.walletAddress1)
	currencyWallet2 := CreateTokenId(&s.walletAddress2)

	s.createCurrencyForWallet(currencyWallet1, big.NewInt(1_000_000), "wallet1", false)
	s.createCurrencyForWallet(currencyWallet2, big.NewInt(2_000_000), "wallet2", false)

	// testAddress1_0 is in the same shard as Minter, thus withdrawal should be performed through sync call
	data, err = s.abiMinter.Pack("withdraw", currencyWallet1.idInt, big.NewInt(1000), s.testAddress1_0)
	s.Require().NoError(err)
	receipt := s.sendMessageViaWallet(s.walletAddress1, types.MinterAddress, execution.MainPrivateKey, data, types.Value{})
	s.Require().True(receipt.Success)
	s.Require().Len(receipt.OutReceipts, 1)
	s.Require().True(receipt.OutReceipts[0].Success)
	// One receipt is for refund
	s.Require().Len(receipt.OutReceipts[0].OutReceipts, 1)

	// walletAddress2 is in a shard other than Minter, thus withdrawal should be performed through async call
	data, err = s.abiMinter.Pack("withdraw", currencyWallet1.idInt, big.NewInt(1000), s.walletAddress2)
	s.Require().NoError(err)
	receipt = s.sendMessageViaWallet(s.walletAddress1, types.MinterAddress, execution.MainPrivateKey, data, types.Value{})
	s.Require().True(receipt.Success)
	s.Require().Len(receipt.OutReceipts, 1)
	s.Require().True(receipt.OutReceipts[0].Success)
	s.Require().Len(receipt.OutReceipts[0].OutReceipts, 1)
	s.Require().True(receipt.OutReceipts[0].OutReceipts[0].Success)

	// Test getName
	data, err = s.abiMinter.Pack("getName", currencyWallet1.idInt)
	s.Require().NoError(err)

	data = s.CallGetter(types.MinterAddress, data, "latest")
	unpackedRes, err := s.abiMinter.Unpack("getName", data)
	s.Require().NoError(err)
	s.Require().Equal("wallet1", unpackedRes[0])

	// Test getIdByName returns correct id
	data, err = s.abiMinter.Pack("getIdByName", "wallet2")
	s.Require().NoError(err)

	data = s.CallGetter(types.MinterAddress, data, "latest")
	unpackedRes, err = s.abiMinter.Unpack("getIdByName", data)
	s.Require().NoError(err)
	s.Require().Equal(currencyWallet2.idInt, unpackedRes[0])

	// Check that getIdByName returns 0 for non-existent currency
	data, err = s.abiMinter.Pack("getIdByName", "not_existing")
	s.Require().NoError(err)

	data = s.CallGetter(types.MinterAddress, data, "latest")
	unpackedRes, err = s.abiMinter.Unpack("getIdByName", data)
	s.Require().NoError(err)
	resInt, ok := unpackedRes[0].(*big.Int)
	s.Require().True(ok)
	s.Require().Zero(resInt.Cmp(big.NewInt(0)))
}

func (s *SuiteMultiCurrencyRpc) createCurrencyForTestContract(currency *CurrencyId, amount types.Value, name string) {
	s.T().Helper()

	data, err := s.abiTest.Pack("createToken", amount.ToBig(), name)
	s.Require().NoError(err)

	txhash, err := s.client.SendExternalMessage(data, *currency.address, nil)
	s.Require().NoError(err)
	receipt := s.waitForReceipt(currency.address.ShardId(), txhash)
	s.Require().True(receipt.Success)
	// If currency address is in the same shard as Minter, then withdrawal will be performed through sync call
	if currency.address.ShardId() == types.MinterAddress.ShardId() {
		s.Require().Empty(receipt.OutReceipts)
	} else {
		s.Require().Len(receipt.OutReceipts, 1)
		s.Require().True(receipt.OutReceipts[0].Success)
	}

	// Check currency is created and balance is correct
	currencies, err := s.client.GetCurrencies(*currency.address, "latest")
	s.Require().NoError(err)
	s.Require().Len(currencies, 1)
	s.Equal(amount, currencies[currency.idStr])
}

func (s *SuiteMultiCurrencyRpc) createCurrencyForWallet(currency *CurrencyId, amount *big.Int, name string, withdraw bool) {
	s.T().Helper()

	data, err := s.abiWallet.Pack("createToken", amount, name, withdraw)
	s.Require().NoError(err)

	receipt := s.sendExternalMessage(data, *currency.address)
	s.Require().True(receipt.Success)
	s.Require().True(receipt.OutReceipts[0].Success)

	if withdraw {
		// Check currency is created and balance is correct
		currencies, err := s.client.GetCurrencies(*currency.address, "latest")
		s.Require().NoError(err)
		s.Require().Equal(amount, currencies[currency.idStr].ToBig())
	}
}

type CurrencyId struct {
	address *types.Address
	id      *types.CurrencyId
	idStr   string
	idInt   *big.Int
}

func CreateTokenId(address *types.Address) *CurrencyId {
	return &CurrencyId{
		address: address,
		id:      types.CurrencyIdForAddress(*address),
		idStr:   hexutil.ToHexNoLeadingZeroes(address.Bytes()),
		idInt:   new(big.Int).SetBytes(address.Bytes()),
	}
}

func TestMultiCurrencyRpc(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteMultiCurrencyRpc))
}
