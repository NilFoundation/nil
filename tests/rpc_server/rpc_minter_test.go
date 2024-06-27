package rpctest

import (
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/NilFoundation/nil/cmd/nil/nilservice"
	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/hexutil"
	"github.com/NilFoundation/nil/contracts"
	"github.com/NilFoundation/nil/core/collate"
	"github.com/NilFoundation/nil/core/execution"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/rpc/jsonrpc"
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
}

func (s *SuiteMultiCurrencyRpc) SetupSuite() {
	s.shardsNum = 4

	var err error
	s.walletAddress1, err = contracts.CalculateAddress("Wallet", 2, []any{execution.MainPublicKey}, []byte{0})
	s.Require().NoError(err)

	s.walletAddress2, err = contracts.CalculateAddress("Wallet", 3, []any{execution.MainPublicKey}, []byte{1})
	s.Require().NoError(err)

	s.walletAddress3, err = contracts.CalculateAddress("Wallet", 3, []any{execution.MainPublicKey}, []byte{3})
	s.Require().NoError(err)

	s.testAddress1_0, err = contracts.CalculateAddress("tests/TokensTest", types.MinterAddress.ShardId(), nil, []byte{1})
	s.Require().NoError(err)

	s.testAddress1_1, err = contracts.CalculateAddress("tests/TokensTest", types.MinterAddress.ShardId(), nil, []byte{2})
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
	zerostate, err := common.ParseTemplate(zerostateTmpl, map[string]interface{}{
		"MinterAddress":        types.MinterAddress.Hex(),
		"MainPublicKey":        hexutil.Encode(execution.MainPublicKey),
		"TestAddress1":         s.walletAddress1.Hex(),
		"TestAddress2":         s.walletAddress2.Hex(),
		"TestAddress3":         s.walletAddress3.Hex(),
		"TokensTestAddress1_0": s.testAddress1_0.Hex(),
		"TokensTestAddress1_1": s.testAddress1_1.Hex(),
	})
	s.Require().NoError(err)

	s.abiMinter, err = contracts.GetAbi("Minter")
	s.Require().NoError(err)

	s.abiTest, err = contracts.GetAbi("tests/TokensTest")
	s.Require().NoError(err)

	s.start(&nilservice.Config{
		NShards:              s.shardsNum,
		HttpPort:             8534,
		Topology:             collate.TrivialShardTopologyId,
		ZeroState:            zerostate,
		CollatorTickPeriodMs: 100,
		GracefulShutdown:     false,
		GasPriceScale:        0,
		GasBasePrice:         10,
	})
	s.waitZerostate()
}

// This test seems to quite big and complex, but there is no obvious way how to split it.
func (s *SuiteMultiCurrencyRpc) TestMultiCurrency() { //nolint
	var (
		data       []byte
		receipt    *jsonrpc.RPCReceipt
		currencies types.CurrenciesMap
		txhash     common.Hash
		err        error
	)
	multiCurrAbi, err := contracts.GetAbi("NilCurrencyBase")
	s.Require().NoError(err)

	currency1 := CreateTokenId(&s.walletAddress1)
	currency2 := CreateTokenId(&s.walletAddress2)

	///////////////////////////////////////////////////////////////////////////
	// Create currency
	data, err = multiCurrAbi.Pack("createToken", big.NewInt(100), "token1", false)
	s.Require().NoError(err)

	receipt = s.sendExternalMessage(data, s.walletAddress1)
	s.Require().True(receipt.Success)
	s.Require().True(receipt.OutReceipts[0].Success)

	// Check currency is created and balance is correct
	currencies, err = s.client.GetCurrencies(types.MinterAddress, "latest")
	s.Require().NoError(err)
	s.Require().Len(currencies, 1)
	s.Require().Equal(types.NewUint256(100), currencies[currency1.idStr])

	// Check currency name is valid
	data, err = s.abiMinter.Pack("getName", &currency1.idInt)
	s.Require().NoError(err)
	data = s.CallGetter(types.MinterAddress, data)
	data = hexutil.FromHex(string(data))
	nameRes, err := s.abiMinter.Unpack("getName", data)
	s.Require().NoError(err)
	name, ok := nameRes[0].(string)
	s.Require().True(ok)
	s.Require().Equal("token1", name)

	///////////////////////////////////////////////////////////////////////////
	// Mint some currency
	data, err = multiCurrAbi.Pack("mintToken", big.NewInt(250), false)
	s.Require().NoError(err)

	receipt = s.sendExternalMessage(data, s.walletAddress1)
	s.Require().True(receipt.Success)
	s.Require().True(receipt.OutReceipts[0].Success)

	// Check currency is minted
	currencies, err = s.client.GetCurrencies(types.MinterAddress, "latest")
	s.Require().NoError(err)
	s.Require().Len(currencies, 1)
	s.Require().Equal(types.NewUint256(350), currencies[currency1.idStr])

	///////////////////////////////////////////////////////////////////////////
	// Transfer some currency
	data, err = multiCurrAbi.Pack("withdrawToken", big.NewInt(100), s.walletAddress1)
	s.Require().NoError(err)

	receipt = s.sendExternalMessage(data, s.walletAddress1)
	s.Require().True(receipt.Success)
	s.Require().True(receipt.OutReceipts[0].Success)

	for !receipt.IsComplete() {
		time.Sleep(100 * time.Millisecond)
	}

	fmt.Printf("receipt: %v\n", receipt)

	// Check that currency has been transferred
	currencies, err = s.client.GetCurrencies(types.MinterAddress, "latest")
	s.Require().NoError(err)
	s.Require().Len(currencies, 1)
	s.Require().Equal(types.NewUint256(250), currencies[currency1.idStr])

	currencies, err = s.client.GetCurrencies(s.walletAddress1, "latest")
	s.Require().NoError(err)
	s.Require().Len(currencies, 1)
	s.Require().Equal(types.NewUint256(100), currencies[currency1.idStr])

	///////////////////////////////////////////////////////////////////////////
	// Send from Wallet1 to Wallet2
	receipt = s.sendMessageViaWalletNoCheck(s.walletAddress1, s.walletAddress2, execution.MainPrivateKey, nil,
		uint256.NewInt(100_000), uint256.NewInt(2_000_000),
		[]types.CurrencyBalance{{Currency: *currency1.id, Balance: *types.NewUint256(40)}})
	s.Require().True(receipt.Success)
	s.Require().True(receipt.OutReceipts[0].Success)

	// Check that currency was transferred
	currencies, err = s.client.GetCurrencies(s.walletAddress1, "latest")
	s.Require().NoError(err)
	s.Require().Len(currencies, 1)
	s.Require().Equal(types.NewUint256(60), currencies[currency1.idStr])

	currencies, err = s.client.GetCurrencies(s.walletAddress2, "latest")
	s.Require().NoError(err)
	s.Require().Len(currencies, 1)
	s.Require().Equal(types.NewUint256(40), currencies[currency1.idStr])

	///////////////////////////////////////////////////////////////////////////
	// Create same currency from different address - should fail
	data, err = s.abiMinter.Pack("create", big.NewInt(100), s.walletAddress1, "token1", types.EmptyAddress)
	s.Require().NoError(err)

	receipt = s.sendMessageViaWallet(s.walletAddress2, types.MinterAddress, execution.MainPrivateKey, data, types.NewUint256(0))
	s.Require().True(receipt.Success)
	s.Require().False(receipt.OutReceipts[0].Success)

	///////////////////////////////////////////////////////////////////////////
	// Create 2-nd currency from Wallet2
	amount := uint256.NewInt(0)
	s.Require().NoError(amount.UnmarshalText([]byte("1000000000000000000000")))
	data, err = s.abiMinter.Pack("create", amount.ToBig(), s.walletAddress2, "token2", types.EmptyAddress)
	s.Require().NoError(err)

	receipt = s.sendMessageViaWallet(s.walletAddress2, types.MinterAddress, execution.MainPrivateKey, data, types.NewUint256(0))
	s.Require().True(receipt.Success)
	s.Require().True(receipt.OutReceipts[0].Success)

	// Check currency is created and balance is correct
	currencies, err = s.client.GetCurrencies(types.MinterAddress, "latest")
	s.Require().NoError(err)
	s.Require().Len(currencies, 2)
	s.Require().Equal(types.NewUint256(250), currencies[currency1.idStr])
	s.Require().Equal(*amount, currencies[currency2.idStr].Int)

	///////////////////////////////////////////////////////////////////////////
	// Transfer all 2-nd currency to Wallet2
	data, err = s.abiMinter.Pack("withdraw", &currency2.idInt, amount.ToBig(), s.walletAddress2)
	s.Require().NoError(err)

	receipt = s.sendMessageViaWallet(s.walletAddress2, types.MinterAddress, execution.MainPrivateKey, data, types.NewUint256(968650))
	s.Require().True(receipt.Success)
	s.Require().True(receipt.OutReceipts[0].Success)

	// Check that currency has been transferred
	currencies, err = s.client.GetCurrencies(types.MinterAddress, "latest")
	s.Require().NoError(err)
	s.Require().Len(currencies, 2)
	s.Require().Zero(currencies[currency1.idStr].Cmp(uint256.NewInt(250)))
	s.Require().Zero(currencies[currency2.idStr].Cmp(uint256.NewInt(0)))

	currencies, err = s.client.GetCurrencies(s.walletAddress2, "latest")
	s.Require().NoError(err)
	s.Require().Len(currencies, 2)
	s.Require().Equal(types.NewUint256(40), currencies[currency1.idStr])
	s.Require().Equal(*amount, currencies[currency2.idStr].Int)

	///////////////////////////////////////////////////////////////////////////
	// Send 1-st and 2-nd currencies Wallet2 to Wallet3 (same shard)
	s.Require().Equal(s.walletAddress2.ShardId(), s.walletAddress3.ShardId())
	receipt = s.sendMessageViaWalletNoCheck(s.walletAddress2, s.walletAddress3, execution.MainPrivateKey, nil,
		uint256.NewInt(1_000_000), uint256.NewInt(2_000_000),
		[]types.CurrencyBalance{
			{Currency: *currency1.id, Balance: *types.NewUint256(10)},
			{Currency: *currency2.id, Balance: *types.NewUint256(500)},
		})
	s.Require().True(receipt.Success)
	s.Require().True(receipt.OutReceipts[0].Success)

	// Check both currencies were transferred
	currencies, err = s.client.GetCurrencies(s.walletAddress3, "latest")
	s.Require().NoError(err)
	s.Require().Len(currencies, 2)
	s.Require().Equal(types.NewUint256(10), currencies[currency1.idStr])
	s.Require().Equal(types.NewUint256(500), currencies[currency2.idStr])

	currencies, err = s.client.GetCurrencies(s.walletAddress2, "latest")
	s.Require().NoError(err)
	s.Require().Len(currencies, 2)
	s.Require().Equal(types.NewUint256(30), currencies[currency1.idStr])
	s.Require().Zero(amount.Sub(amount, uint256.NewInt(500)).Cmp(&currencies[currency2.idStr].Int))

	///////////////////////////////////////////////////////////////////////////
	// Transfer 1-nd currency to Wallet2 - should fail, wrong owner
	data, err = s.abiMinter.Pack("withdraw", &currency1.idInt, big.NewInt(2), s.walletAddress2)
	s.Require().NoError(err)

	receipt = s.sendMessageViaWallet(s.walletAddress2, types.MinterAddress, execution.MainPrivateKey, data, types.NewUint256(0))
	s.Require().True(receipt.Success)
	s.Require().False(receipt.OutReceipts[0].Success)

	///////////////////////////////////////////////////////////////////////////
	// Send insufficient amount of 1-nd currency - should fail
	receipt = s.sendMessageViaWalletNoCheck(s.walletAddress2, s.walletAddress3, execution.MainPrivateKey, nil,
		uint256.NewInt(1_000_000), uint256.NewInt(2_000_000),
		[]types.CurrencyBalance{{Currency: *currency1.id, Balance: *types.NewUint256(700)}})
	s.Require().False(receipt.Success)

	// Check that currency was not sent
	currencies, err = s.client.GetCurrencies(s.walletAddress2, "latest")
	s.Require().NoError(err)
	s.Require().Len(currencies, 2)
	s.Require().Equal(types.NewUint256(30), currencies[currency1.idStr])

	currencies, err = s.client.GetCurrencies(s.walletAddress3, "latest")
	s.Require().NoError(err)
	s.Require().Len(currencies, 2)
	s.Require().Equal(types.NewUint256(10), currencies[currency1.idStr])

	///////////////////////////////////////////////////////////////////////////
	// Mint some currency and transfer it to wallet3 at once
	currencies, err = s.client.GetCurrencies(types.MinterAddress, "latest")
	s.Require().NoError(err)
	minterCurrency1 := currencies[currency1.idStr]
	currencies, err = s.client.GetCurrencies(s.walletAddress1, "latest")
	s.Require().NoError(err)
	walletCurrency1 := currencies[currency1.idStr]

	data, err = s.abiMinter.Pack("mint", &currency1.idInt, big.NewInt(1000), s.walletAddress3)
	s.Require().NoError(err)

	receipt = s.sendMessageViaWallet(s.walletAddress1, types.MinterAddress, execution.MainPrivateKey, data, types.NewUint256(0))
	s.Require().True(receipt.Success)
	s.Require().True(receipt.OutReceipts[0].Success)

	// Check currency is minted, minter should not get it
	currencies, err = s.client.GetCurrencies(types.MinterAddress, "latest")
	s.Require().NoError(err)
	s.Require().Equal(minterCurrency1, currencies[currency1.idStr])

	// Currency of wallet1(owner) should not be changed
	currencies, err = s.client.GetCurrencies(s.walletAddress1, "latest")
	s.Require().NoError(err)
	s.Require().Equal(walletCurrency1, currencies[currency1.idStr])

	// Finally currency should be credited to wallet3
	currencies, err = s.client.GetCurrencies(s.walletAddress3, "latest")
	s.Require().NoError(err)
	s.Require().Equal(types.NewUint256(1010), currencies[currency1.idStr])

	///////////////////////////////////////////////////////////////////////////
	// Second part of testing: tests through TokensTest.sol
	//
	currencyTest1 := CreateTokenId(&s.testAddress1_0)
	currencyTest2 := CreateTokenId(&s.testAddress1_1)

	///////////////////////////////////////////////////////////////////////////
	// Create tokens for test addresses
	s.createCurrency(currencyTest1, big.NewInt(1_000_000), "testToken1")
	s.createCurrency(currencyTest2, big.NewInt(2_000_000), "testToken2")

	///////////////////////////////////////////////////////////////////////////
	// Call testCallWithTokensSync of testAddress1_0, which should call method of testAddress1_1
	data, err = s.abiTest.Pack("testCallWithTokensSync", s.testAddress1_1,
		[]types.CurrencyBalanceAbiCompatible{{Currency: currencyTest1.idInt, Balance: uint256.NewInt(5000).ToBig()}})
	s.Require().NoError(err)

	txhash, err = s.client.SendExternalMessage(data, s.testAddress1_0, nil)
	s.Require().NoError(err)
	receipt = s.waitForReceipt(s.testAddress1_0.ShardId(), txhash)
	s.Require().True(receipt.Success)

	// Check currency is debited from testAddress1_0
	currencies, err = s.client.GetCurrencies(s.testAddress1_0, "latest")
	s.Require().NoError(err)
	s.Require().Equal(types.NewUint256(1_000_000-5000), currencies[currencyTest1.idStr])

	// Check currency is credited to testAddress1_1
	currencies, err = s.client.GetCurrencies(s.testAddress1_1, "latest")
	s.Require().NoError(err)
	s.Require().Equal(types.NewUint256(5000), currencies[currencyTest1.idStr])

	///////////////////////////////////////////////////////////////////////////
	// Try to call with non existent currency
	data, err = s.abiTest.Pack("testCallWithTokensSync", s.testAddress1_1,
		[]types.CurrencyBalanceAbiCompatible{
			{Currency: currencyTest1.idInt, Balance: uint256.NewInt(5000).ToBig()},
			{Currency: big.NewInt(0).Add(currencyTest1.idInt, big.NewInt(1)), Balance: uint256.NewInt(1).ToBig()},
		})
	s.Require().NoError(err)

	txhash, err = s.client.SendExternalMessage(data, s.testAddress1_0, nil)
	s.Require().NoError(err)
	receipt = s.waitForReceipt(s.testAddress1_0.ShardId(), txhash)
	s.Require().False(receipt.Success)

	// Check currency of testAddress1_0 was not changed
	currencies, err = s.client.GetCurrencies(s.testAddress1_0, "latest")
	s.Require().NoError(err)
	s.Require().Equal(types.NewUint256(1_000_000-5000), currencies[currencyTest1.idStr])

	// Check currency of testAddress1_1 was not changed
	currencies, err = s.client.GetCurrencies(s.testAddress1_1, "latest")
	s.Require().NoError(err)
	s.Require().Equal(types.NewUint256(5000), currencies[currencyTest1.idStr])

	///////////////////////////////////////////////////////////////////////////
	// Call testCallWithTokensAsync of testAddress1_0, which should send message to testAddress1_1
	data, err = s.abiTest.Pack("testCallWithTokensAsync", s.testAddress1_1,
		[]types.CurrencyBalanceAbiCompatible{{Currency: currencyTest1.idInt, Balance: uint256.NewInt(5000).ToBig()}})
	s.Require().NoError(err)

	txhash, err = s.client.SendExternalMessage(data, s.testAddress1_0, nil)
	s.Require().NoError(err)
	receipt = s.waitForReceipt(s.testAddress1_0.ShardId(), txhash)
	s.Require().True(receipt.Success)
	s.Require().Len(receipt.OutReceipts, 1)
	s.Require().True(receipt.OutReceipts[0].Success)

	// Check currency is debited from testAddress1_0
	currencies, err = s.client.GetCurrencies(s.testAddress1_0, "latest")
	s.Require().NoError(err)
	s.Require().Equal(types.NewUint256(1_000_000-5000-5000), currencies[currencyTest1.idStr])

	// Check currency is credited to testAddress1_1
	currencies, err = s.client.GetCurrencies(s.testAddress1_1, "latest")
	s.Require().NoError(err)
	s.Require().Equal(types.NewUint256(5000+5000), currencies[currencyTest1.idStr])

	///////////////////////////////////////////////////////////////////////////
	// Try to call with non existent currency
	data, err = s.abiTest.Pack("testCallWithTokensAsync", s.testAddress1_1,
		[]types.CurrencyBalanceAbiCompatible{
			{Currency: currencyTest1.idInt, Balance: uint256.NewInt(5000).ToBig()},
			{Currency: big.NewInt(0).Add(currencyTest1.idInt, big.NewInt(1)), Balance: uint256.NewInt(1).ToBig()},
		})
	s.Require().NoError(err)

	txhash, err = s.client.SendExternalMessage(data, s.testAddress1_0, nil)
	s.Require().NoError(err)
	receipt = s.waitForReceipt(s.testAddress1_0.ShardId(), txhash)
	s.Require().False(receipt.Success)
	s.Require().Empty(receipt.OutReceipts)

	// Check currency of testAddress1_0 was not changed
	currencies, err = s.client.GetCurrencies(s.testAddress1_0, "latest")
	s.Require().NoError(err)
	s.Require().Equal(types.NewUint256(1_000_000-5000-5000), currencies[currencyTest1.idStr])

	// Check currency of testAddress1_1 was not changed
	currencies, err = s.client.GetCurrencies(s.testAddress1_1, "latest")
	s.Require().NoError(err)
	s.Require().Equal(types.NewUint256(5000+5000), currencies[currencyTest1.idStr])

	///////////////////////////////////////////////////////////////////////////
	// Call method that transfer tokens through sync call
	amountTest1 := s.getCurrencyBalance(&s.testAddress1_0, currencyTest1)
	amountTest2 := s.getCurrencyBalance(&s.testAddress1_1, currencyTest1)
	data, err = s.abiTest.Pack("testSendTokensSync", s.testAddress1_1, big.NewInt(5000), false)
	s.Require().NoError(err)

	txhash, err = s.client.SendExternalMessage(data, s.testAddress1_0, nil)
	s.Require().NoError(err)
	receipt = s.waitForReceipt(s.testAddress1_0.ShardId(), txhash)
	s.Require().True(receipt.Success)
	s.Require().Empty(receipt.OutReceipts)
	s.Require().Empty(receipt.OutMessages)

	// Check currency was debited from testAddress1_0
	currencies, err = s.client.GetCurrencies(s.testAddress1_0, "latest")
	s.Require().NoError(err)
	s.Require().Equal(types.NewUint256(amountTest1-5000), currencies[currencyTest1.idStr])

	// Check currency was credited to testAddress1_1
	currencies, err = s.client.GetCurrencies(s.testAddress1_1, "latest")
	s.Require().NoError(err)
	s.Require().Equal(types.NewUint256(amountTest2+5000), currencies[currencyTest1.idStr])

	///////////////////////////////////////////////////////////////////////////
	// Call the same method but fail flag, so currency should not be changed
	data, err = s.abiTest.Pack("testSendTokensSync", s.testAddress1_1, big.NewInt(5000), true)
	s.Require().NoError(err)

	txhash, err = s.client.SendExternalMessage(data, s.testAddress1_0, nil)
	s.Require().NoError(err)
	receipt = s.waitForReceipt(s.testAddress1_0.ShardId(), txhash)
	s.Require().False(receipt.Success)

	// Check currency of testAddress1_0 was not changed
	currencies, err = s.client.GetCurrencies(s.testAddress1_0, "latest")
	s.Require().NoError(err)
	s.Require().Equal(types.NewUint256(amountTest1-5000), currencies[currencyTest1.idStr])

	// Check currency of testAddress1_1 was not changed
	currencies, err = s.client.GetCurrencies(s.testAddress1_1, "latest")
	s.Require().NoError(err)
	s.Require().Equal(types.NewUint256(amountTest2+5000), currencies[currencyTest1.idStr])

	///////////////////////////////////////////////////////////////////////////
	// Call `testSendTokensSync` for address in different shard - should fail
	data, err = s.abiTest.Pack("testSendTokensSync", s.walletAddress3, big.NewInt(5000), false)
	s.Require().NoError(err)

	txhash, err = s.client.SendExternalMessage(data, s.testAddress1_0, nil)
	s.Require().NoError(err)
	receipt = s.waitForReceipt(s.testAddress1_0.ShardId(), txhash)
	s.Require().False(receipt.Success)

	// Check currency of testAddress1_0 was not changed
	currencies, err = s.client.GetCurrencies(s.testAddress1_0, "latest")
	s.Require().NoError(err)
	s.Require().Equal(types.NewUint256(amountTest1-5000), currencies[currencyTest1.idStr])
}

func (s *SuiteMultiCurrencyRpc) getCurrencyBalance(address *types.Address, currency *CurrencyId) uint64 {
	s.T().Helper()

	currencies, err := s.client.GetCurrencies(*address, "latest")
	s.Require().NoError(err)
	return currencies[currency.idStr].Uint64()
}

func (s *SuiteMultiCurrencyRpc) createCurrency(currency *CurrencyId, amount *big.Int, name string) {
	s.T().Helper()

	data, err := s.abiTest.Pack("createToken", amount, name)
	s.Require().NoError(err)

	txhash, err := s.client.SendExternalMessage(data, *currency.address, nil)
	s.Require().NoError(err)
	receipt := s.waitForReceipt(currency.address.ShardId(), txhash)
	s.Require().True(receipt.Success)
	s.Require().True(receipt.OutReceipts[0].Success)

	// Check currency is created and balance is correct
	currencies, err := s.client.GetCurrencies(*currency.address, "latest")
	s.Require().NoError(err)
	s.Require().Len(currencies, 1)
	v, _ := uint256.FromBig(amount)
	s.Require().Equal(&types.Uint256{Int: *v}, currencies[currency.idStr])
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
