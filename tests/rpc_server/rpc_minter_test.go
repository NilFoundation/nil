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
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/suite"
)

type SuiteMinterRpc struct {
	RpcSuite
	walletAddress1 types.Address
	walletAddress2 types.Address
	walletAddress3 types.Address
}

func currencyIdFromAddress(a types.Address) *types.CurrencyId {
	c := new(types.CurrencyId)
	copy(c[12:], a.Bytes())
	return c
}

func (s *SuiteMinterRpc) SetupSuite() {
	s.shardsNum = 4

	var err error
	s.walletAddress1, err = contracts.CalculateAddress("Wallet", 2, []any{execution.MainPublicKey}, []byte{0})
	s.Require().NoError(err)

	s.walletAddress2, err = contracts.CalculateAddress("Wallet", 3, []any{execution.MainPublicKey}, []byte{1})
	s.Require().NoError(err)

	s.walletAddress3, err = contracts.CalculateAddress("Wallet", 3, []any{execution.MainPublicKey}, []byte{3})
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
`
	zerostate, err := common.ParseTemplate(zerostateTmpl, map[string]interface{}{
		"MinterAddress": types.MinterAddress.Hex(),
		"MainPublicKey": hexutil.Encode(execution.MainPublicKey),
		"TestAddress1":  s.walletAddress1.Hex(),
		"TestAddress2":  s.walletAddress2.Hex(),
		"TestAddress3":  s.walletAddress3.Hex(),
	})
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
}

func (s *SuiteMinterRpc) TestBasic() {
	var (
		data       []byte
		receipt    *jsonrpc.RPCReceipt
		currencies map[string]*types.Uint256
	)

	abiMinter, err := contracts.GetAbi("Minter")
	s.Require().NoError(err)
	multiCurrAbi, err := contracts.GetAbi("NilCurrencyBase")
	s.Require().NoError(err)

	currencyId1 := currencyIdFromAddress(s.walletAddress1)
	currencyIdStr1 := hexutil.ToHexNoLeadingZeroes(s.walletAddress1.Bytes())
	var currencyIdInt1 big.Int
	currencyIdInt1.SetBytes(s.walletAddress1.Bytes())

	currencyId2 := currencyIdFromAddress(s.walletAddress2)
	currencyIdStr2 := hexutil.ToHexNoLeadingZeroes(s.walletAddress2.Bytes())
	var currencyIdInt2 big.Int
	currencyIdInt2.SetBytes(s.walletAddress2.Bytes())

	///////////////////////////////////////////////////////////////////////////
	// Create currency
	data, err = multiCurrAbi.Pack("createToken", big.NewInt(100))
	s.Require().NoError(err)

	receipt = s.sendExternalMessage(data, s.walletAddress1)
	s.Require().True(receipt.Success)
	s.Require().True(receipt.OutReceipts[0].Success)

	// Check currency is created and balance is correct
	currencies, err = s.client.GetCurrencies(types.MinterAddress, "latest")
	s.Require().NoError(err)
	s.Require().Len(currencies, 1)
	s.Require().Equal(types.NewUint256(100), currencies[currencyIdStr1])

	///////////////////////////////////////////////////////////////////////////
	// Mint some currency
	data, err = multiCurrAbi.Pack("mintToken", big.NewInt(250))
	s.Require().NoError(err)

	receipt = s.sendExternalMessage(data, s.walletAddress1)
	s.Require().True(receipt.Success)
	s.Require().True(receipt.OutReceipts[0].Success)

	// Check currency is minted
	currencies, err = s.client.GetCurrencies(types.MinterAddress, "latest")
	s.Require().NoError(err)
	s.Require().Len(currencies, 1)
	s.Require().Equal(types.NewUint256(350), currencies[currencyIdStr1])

	///////////////////////////////////////////////////////////////////////////
	// Transfer some currency
	data, err = multiCurrAbi.Pack("transferToken", big.NewInt(100), s.walletAddress1)
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
	s.Require().Equal(types.NewUint256(250), currencies[currencyIdStr1])

	currencies, err = s.client.GetCurrencies(s.walletAddress1, "latest")
	s.Require().NoError(err)
	s.Require().Len(currencies, 1)
	s.Require().Equal(types.NewUint256(100), currencies[currencyIdStr1])

	///////////////////////////////////////////////////////////////////////////
	// Send from Wallet1 to Wallet2
	receipt = s.sendMessageViaWalletNoCheck(s.walletAddress1, s.walletAddress2, execution.MainPrivateKey, nil,
		uint256.NewInt(100_000), uint256.NewInt(2_000_000),
		[]types.CurrencyBalance{{Currency: *currencyId1, Balance: *types.NewUint256(40)}})
	s.Require().True(receipt.Success)
	s.Require().True(receipt.OutReceipts[0].Success)

	// Check that currency was transferred
	currencies, err = s.client.GetCurrencies(s.walletAddress1, "latest")
	s.Require().NoError(err)
	s.Require().Len(currencies, 1)
	s.Require().Equal(types.NewUint256(60), currencies[currencyIdStr1])

	currencies, err = s.client.GetCurrencies(s.walletAddress2, "latest")
	s.Require().NoError(err)
	s.Require().Len(currencies, 1)
	s.Require().Equal(types.NewUint256(40), currencies[currencyIdStr1])

	///////////////////////////////////////////////////////////////////////////
	// Create same currency from different address - should fail
	data, err = abiMinter.Pack("create", big.NewInt(100), s.walletAddress1)
	s.Require().NoError(err)

	receipt = s.sendMessageViaWallet(s.walletAddress2, types.MinterAddress, execution.MainPrivateKey, data, types.NewUint256(0))
	s.Require().True(receipt.Success)
	s.Require().False(receipt.OutReceipts[0].Success)

	///////////////////////////////////////////////////////////////////////////
	// Create 2-nd currency from Wallet2
	amount := uint256.NewInt(0)
	s.Require().NoError(amount.UnmarshalText([]byte("1000000000000000000000")))
	data, err = abiMinter.Pack("create", amount.ToBig(), s.walletAddress2)
	s.Require().NoError(err)

	receipt = s.sendMessageViaWallet(s.walletAddress2, types.MinterAddress, execution.MainPrivateKey, data, types.NewUint256(0))
	s.Require().True(receipt.Success)
	s.Require().True(receipt.OutReceipts[0].Success)

	// Check currency is created and balance is correct
	currencies, err = s.client.GetCurrencies(types.MinterAddress, "latest")
	s.Require().NoError(err)
	s.Require().Len(currencies, 2)
	s.Require().Equal(types.NewUint256(250), currencies[currencyIdStr1])
	s.Require().Equal(*amount, currencies[currencyIdStr2].Int)

	///////////////////////////////////////////////////////////////////////////
	// Transfer all 2-nd currency to Wallet2
	data, err = abiMinter.Pack("transfer", &currencyIdInt2, amount.ToBig(), s.walletAddress2)
	s.Require().NoError(err)

	receipt = s.sendMessageViaWallet(s.walletAddress2, types.MinterAddress, execution.MainPrivateKey, data, types.NewUint256(968650))
	s.Require().True(receipt.Success)
	s.Require().True(receipt.OutReceipts[0].Success)

	// Check that currency has been transferred
	currencies, err = s.client.GetCurrencies(types.MinterAddress, "latest")
	s.Require().NoError(err)
	s.Require().Len(currencies, 2)
	s.Require().Zero(currencies[currencyIdStr1].Cmp(uint256.NewInt(250)))
	s.Require().Zero(currencies[currencyIdStr2].Cmp(uint256.NewInt(0)))

	currencies, err = s.client.GetCurrencies(s.walletAddress2, "latest")
	s.Require().NoError(err)
	s.Require().Len(currencies, 2)
	s.Require().Equal(types.NewUint256(40), currencies[currencyIdStr1])
	s.Require().Equal(*amount, currencies[currencyIdStr2].Int)

	///////////////////////////////////////////////////////////////////////////
	// Send 1-st and 2-nd currencies Wallet2 to Wallet3 (same shard)
	s.Require().Equal(s.walletAddress2.ShardId(), s.walletAddress3.ShardId())
	receipt = s.sendMessageViaWalletNoCheck(s.walletAddress2, s.walletAddress3, execution.MainPrivateKey, nil,
		uint256.NewInt(1_000_000), uint256.NewInt(2_000_000),
		[]types.CurrencyBalance{
			{Currency: *currencyId1, Balance: *types.NewUint256(10)},
			{Currency: *currencyId2, Balance: *types.NewUint256(500)},
		})
	s.Require().True(receipt.Success)
	s.Require().True(receipt.OutReceipts[0].Success)

	// Check both currencies were transferred
	currencies, err = s.client.GetCurrencies(s.walletAddress3, "latest")
	s.Require().NoError(err)
	s.Require().Len(currencies, 2)
	s.Require().Equal(types.NewUint256(10), currencies[currencyIdStr1])
	s.Require().Equal(types.NewUint256(500), currencies[currencyIdStr2])

	currencies, err = s.client.GetCurrencies(s.walletAddress2, "latest")
	s.Require().NoError(err)
	s.Require().Len(currencies, 2)
	s.Require().Equal(types.NewUint256(30), currencies[currencyIdStr1])
	s.Require().Zero(amount.Sub(amount, uint256.NewInt(500)).Cmp(&currencies[currencyIdStr2].Int))

	///////////////////////////////////////////////////////////////////////////
	// Transfer 1-nd currency to Wallet2 - should fail, wrong owner
	data, err = abiMinter.Pack("transfer", &currencyIdInt1, big.NewInt(2), s.walletAddress2)
	s.Require().NoError(err)

	receipt = s.sendMessageViaWallet(s.walletAddress2, types.MinterAddress, execution.MainPrivateKey, data, types.NewUint256(0))
	s.Require().True(receipt.Success)
	s.Require().False(receipt.OutReceipts[0].Success)

	///////////////////////////////////////////////////////////////////////////
	// Send insufficient amount of 1-nd currency - should fail
	receipt = s.sendMessageViaWalletNoCheck(s.walletAddress2, s.walletAddress3, execution.MainPrivateKey, nil,
		uint256.NewInt(1_000_000), uint256.NewInt(2_000_000),
		[]types.CurrencyBalance{{Currency: *currencyId1, Balance: *types.NewUint256(700)}})
	s.Require().False(receipt.Success)

	// Check that currency was not sent
	currencies, err = s.client.GetCurrencies(s.walletAddress2, "latest")
	s.Require().NoError(err)
	s.Require().Len(currencies, 2)
	s.Require().Equal(types.NewUint256(30), currencies[currencyIdStr1])

	currencies, err = s.client.GetCurrencies(s.walletAddress3, "latest")
	s.Require().NoError(err)
	s.Require().Len(currencies, 2)
	s.Require().Equal(types.NewUint256(10), currencies[currencyIdStr1])
}

func TestSuiteMinterRpc(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteMinterRpc))
}
