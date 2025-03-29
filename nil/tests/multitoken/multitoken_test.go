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

type SuiteMultiTokenRpc struct {
	tests.RpcSuite
	smartAccountAddress1 types.Address
	smartAccountAddress2 types.Address
	smartAccountAddress3 types.Address
	testAddress1_0       types.Address
	testAddress1_1       types.Address
	testAddressNoAccess  types.Address
	abiTest              *abi.ABI
	abiSmartAccount      *abi.ABI
}

func (s *SuiteMultiTokenRpc) SetupSuite() {
	s.ShardsNum = 4

	s.smartAccountAddress1 = contracts.SmartAccountAddress(s.T(), 2, []byte{0}, execution.MainPublicKey)
	s.smartAccountAddress2 = contracts.SmartAccountAddress(s.T(), 3, []byte{1}, execution.MainPublicKey)
	s.smartAccountAddress3 = contracts.SmartAccountAddress(s.T(), 3, []byte{3}, execution.MainPublicKey)

	var err error
	s.testAddress1_0, err = contracts.CalculateAddress(contracts.NameTokensTest, 1, []byte{1})
	s.Require().NoError(err)

	s.testAddress1_1, err = contracts.CalculateAddress(contracts.NameTokensTest, 1, []byte{2})
	s.Require().NoError(err)

	s.testAddressNoAccess, err = contracts.CalculateAddress(contracts.NameTokensTestNoExternalAccess, 1, nil)
	s.Require().NoError(err)

	s.abiSmartAccount, err = contracts.GetAbi("SmartAccount")
	s.Require().NoError(err)

	s.abiTest, err = contracts.GetAbi(contracts.NameTokensTest)
	s.Require().NoError(err)
}

func (s *SuiteMultiTokenRpc) SetupTest() {
	smartAccountValue, err := types.NewValueFromDecimal("100000000000000")
	s.Require().NoError(err)
	zerostateCfg := &execution.ZeroStateConfig{
		Contracts: []*execution.ContractDescr{
			{
				Name:     "TestSmartAccountShard2",
				Contract: "SmartAccount",
				Address:  s.smartAccountAddress1,
				Value:    smartAccountValue,
				CtorArgs: []any{execution.MainPublicKey},
			},
			{
				Name:     "TestSmartAccountShard3",
				Contract: "SmartAccount",
				Address:  s.smartAccountAddress2,
				Value:    smartAccountValue,
				CtorArgs: []any{execution.MainPublicKey},
			},
			{
				Name:     "TestSmartAccountShard3a",
				Contract: "SmartAccount",
				Address:  s.smartAccountAddress3,
				Value:    smartAccountValue,
				CtorArgs: []any{execution.MainPublicKey},
			},
			{
				Name:     "TokensTest1_0",
				Contract: contracts.NameTokensTest,
				Address:  s.testAddress1_0,
				Value:    smartAccountValue,
			},
			{
				Name:     "TokensTest1_1",
				Contract: contracts.NameTokensTest,
				Address:  s.testAddress1_1,
				Value:    smartAccountValue,
			},
			{
				Name:     "TokensTestNoAccess",
				Contract: contracts.NameTokensTestNoExternalAccess,
				Address:  s.testAddressNoAccess,
				Value:    smartAccountValue,
			},
		},
	}

	s.Start(&nilservice.Config{
		NShards:   s.ShardsNum,
		HttpUrl:   rpc.GetSockPath(s.T()),
		ZeroState: zerostateCfg,
		RunMode:   nilservice.CollatorsOnlyRunMode,
	})
}

func (s *SuiteMultiTokenRpc) TearDownTest() {
	s.Cancel()
}

// This test seems to quite big and complex, but there is no obvious way how to split it.
func (s *SuiteMultiTokenRpc) TestMultiToken() { //nolint

	token1 := CreateTokenId(&s.smartAccountAddress1)
	token2 := CreateTokenId(&s.smartAccountAddress2)

	s.Run("Initialize token", func() {
		data := s.AbiPack(s.abiSmartAccount, "setTokenName", "token1")
		receipt := s.SendExternalTransactionNoCheck(data, s.smartAccountAddress1)
		s.Require().True(receipt.Success)

		data = s.AbiPack(s.abiSmartAccount, "mintToken", big.NewInt(100))
		receipt = s.SendExternalTransactionNoCheck(data, s.smartAccountAddress1)
		s.Require().True(receipt.Success)

		s.Run("Check token is initialized", func() {
			tokens, err := s.Client.GetTokens(s.Context, s.smartAccountAddress1, "latest")
			s.Require().NoError(err)
			s.Require().Len(tokens, 1)
			s.Equal(types.NewValueFromUint64(100), tokens[*token1.id])
		})

		s.Run("Check token name", func() {
			data := s.AbiPack(s.abiSmartAccount, "getTokenName")
			data = s.CallGetter(s.smartAccountAddress1, data, "latest", nil)
			nameRes := s.AbiUnpack(s.abiSmartAccount, "getTokenName", data)
			name, ok := nameRes[0].(string)
			s.Require().True(ok)
			s.Require().Equal("token1", name)
		})

		s.Run("Check token total supply", func() {
			data := s.AbiPack(s.abiSmartAccount, "getTokenTotalSupply")
			data = s.CallGetter(s.smartAccountAddress1, data, "latest", nil)
			results := s.AbiUnpack(s.abiSmartAccount, "getTokenTotalSupply", data)
			totalSupply, ok := results[0].(*big.Int)
			s.Require().True(ok)
			s.Require().Equal(big.NewInt(100), totalSupply)
		})
	})

	checkManageToken := func(method string, arg int64, balance int64) {
		s.T().Helper()

		s.Run(method+" token", func() {
			data, err := s.abiSmartAccount.Pack(method+"Token", big.NewInt(arg))
			s.Require().NoError(err)

			receipt := s.SendExternalTransactionNoCheck(data, s.smartAccountAddress1)
			s.Require().True(receipt.Success)

			s.Run(fmt.Sprintf("Check token is %sed", method), func() {
				tokens, err := s.Client.GetTokens(s.Context, s.smartAccountAddress1, "latest")
				s.Require().NoError(err)
				s.Require().Len(tokens, 1)
				s.Equal(types.NewValueFromUint64(uint64(balance)), tokens[*token1.id])
			})

			s.Run("Check token total supply", func() {
				data := s.AbiPack(s.abiSmartAccount, "getTokenTotalSupply")
				data = s.CallGetter(s.smartAccountAddress1, data, "latest", nil)
				results := s.AbiUnpack(s.abiSmartAccount, "getTokenTotalSupply", data)
				totalSupply, ok := results[0].(*big.Int)
				s.Require().True(ok)
				s.Require().Equal(big.NewInt(balance), totalSupply)
			})
		})
	}

	checkManageToken("mint", 350, 450)

	checkManageToken("burn", 100, 350)

	s.Run("Transfer token via sendToken", func() {
		data := s.AbiPack(s.abiSmartAccount, "sendToken", s.smartAccountAddress2, *token1.id, big.NewInt(100))

		receipt := s.SendExternalTransaction(data, s.smartAccountAddress1)
		s.Require().True(receipt.Success)
		s.Require().True(receipt.OutReceipts[0].Success)

		s.Run("Check token is transferred", func() {
			tokens, err := s.Client.GetTokens(s.Context, s.smartAccountAddress1, "latest")
			s.Require().NoError(err)
			s.Require().Len(tokens, 1)
			s.Equal(types.NewValueFromUint64(250), tokens[*token1.id])

			tokens, err = s.Client.GetTokens(s.Context, s.smartAccountAddress2, "latest")
			s.Require().NoError(err)
			s.Require().Len(tokens, 1)
			s.Equal(types.NewValueFromUint64(100), tokens[*token1.id])
		})
	})

	s.Run("Send from Wallet1 to Wallet2 via asyncCall", func() {
		receipt := s.SendTransactionViaSmartAccountNoCheck(
			s.smartAccountAddress1,
			s.smartAccountAddress2,
			execution.MainPrivateKey,
			nil,
			types.NewFeePackFromGas(500_000),
			types.Value{},
			[]types.TokenBalance{{Token: *token1.id, Balance: types.NewValueFromUint64(50)}})
		s.Require().True(receipt.Success)
		s.Require().True(receipt.OutReceipts[0].Success)

		s.Run("Check token is transferred", func() {
			tokens, err := s.Client.GetTokens(s.Context, s.smartAccountAddress1, "latest")
			s.Require().NoError(err)
			s.Require().Len(tokens, 1)
			s.Equal(types.NewValueFromUint64(200), tokens[*token1.id])

			tokens, err = s.Client.GetTokens(s.Context, s.smartAccountAddress2, "latest")
			s.Require().NoError(err)
			s.Require().Len(tokens, 1)
			s.Equal(types.NewValueFromUint64(150), tokens[*token1.id])

			// Cross-shard `Nil.tokenBalance` should fail
			s.Require().NotEqual(s.testAddress1_0.ShardId(), s.smartAccountAddress2.ShardId())
			data := s.AbiPack(s.abiTest, "checkTokenBalance", s.smartAccountAddress2, token1.id, big.NewInt(150))
			receipt = s.SendExternalTransactionNoCheck(data, s.testAddress1_0)
			s.Require().False(receipt.Success)
		})
	})

	s.Run("TestDeployWithToken", func() {
		tokens := []types.TokenBalance{{Token: *token1.id, Balance: types.NewValueFromUint64(10)}}
		contractCode, _ := s.LoadContract(common.GetAbsolutePath("../contracts/increment.sol"), "Incrementer")

		data, err := s.abiTest.Pack("testAsyncDeployWithTokens",
			types.NewValueFromUint64(uint64(types.BaseShardId)),
			types.NewFeePackFromGas(500_000).FeeCredit,
			types.Value0,
			[]byte(contractCode),
			types.Value0,
			tokens,
		)
		s.Require().NoError(err)

		hash, err := s.Client.SendExternalTransaction(
			s.Context, data, s.testAddress1_0, nil, types.NewFeePackFromGas(100_000))
		s.Require().NoError(err)
		receipt := s.WaitForReceipt(hash)
		s.Require().False(receipt.Success)
		s.Require().Equal("AsyncDeployMustNotHaveToken", receipt.Status)
	})

	var amount types.Value
	s.Require().NoError(amount.Set("1000000000000000000000"))

	s.Run("Create 2-nd token from SmartAccount2", func() {
		data := s.AbiPack(s.abiSmartAccount, "setTokenName", "token2")
		receipt := s.SendExternalTransactionNoCheck(data, s.smartAccountAddress2)
		s.Require().True(receipt.Success)

		data = s.AbiPack(s.abiSmartAccount, "mintToken", amount.ToBig())
		receipt = s.SendExternalTransactionNoCheck(data, s.smartAccountAddress2)
		s.Require().True(receipt.Success)

		s.Run("Check token and balance", func() {
			tokens, err := s.Client.GetTokens(s.Context, s.smartAccountAddress2, "latest")
			s.Require().NoError(err)
			s.Require().Len(tokens, 2)
			s.Equal(types.NewValueFromUint64(150), tokens[*token1.id])
			s.Equal(amount, tokens[*token2.id])
		})
	})

	s.Run("Send 1-st and 2-nd tokens from Wallet2 to Wallet3 (same shard)", func() {
		s.Require().Equal(s.smartAccountAddress2.ShardId(), s.smartAccountAddress3.ShardId())
		receipt := s.SendTransactionViaSmartAccountNoCheck(
			s.smartAccountAddress2,
			s.smartAccountAddress3,
			execution.MainPrivateKey,
			nil,
			types.NewFeePackFromGas(500_000),
			types.Value{},
			[]types.TokenBalance{
				{Token: *token1.id, Balance: types.NewValueFromUint64(10)},
				{Token: *token2.id, Balance: types.NewValueFromUint64(500)},
			})
		s.Require().True(receipt.Success)
		s.Require().True(receipt.OutReceipts[0].Success)

		s.Run("Check tokens are transferred", func() {
			tokens, err := s.Client.GetTokens(s.Context, s.smartAccountAddress3, "latest")
			s.Require().NoError(err)
			s.Require().Len(tokens, 2)
			s.Equal(types.NewValueFromUint64(10), tokens[*token1.id])
			s.Equal(types.NewValueFromUint64(500), tokens[*token2.id])

			tokens, err = s.Client.GetTokens(s.Context, s.smartAccountAddress2, "latest")
			s.Require().NoError(err)
			s.Require().Len(tokens, 2)
			s.Equal(types.NewValueFromUint64(140), tokens[*token1.id])
			s.Equal(amount.Sub(types.NewValueFromUint64(500)), tokens[*token2.id])
		})
	})

	s.Run("Fail to send insufficient amount of 1st token", func() {
		receipt := s.SendTransactionViaSmartAccountNoCheck(
			s.smartAccountAddress2,
			s.smartAccountAddress3,
			execution.MainPrivateKey,
			nil,
			types.NewFeePackFromGas(100_000),
			types.Value{},
			[]types.TokenBalance{{Token: *token1.id, Balance: types.NewValueFromUint64(700)}})
		s.Require().False(receipt.Success)
		s.Require().Contains(receipt.ErrorMessage, vm.ErrInsufficientBalance.Error())

		s.Run("Check token is not sent", func() {
			tokens, err := s.Client.GetTokens(s.Context, s.smartAccountAddress2, "latest")
			s.Require().NoError(err)
			s.Require().Len(tokens, 2)
			s.Equal(types.NewValueFromUint64(140), tokens[*token1.id])

			tokens, err = s.Client.GetTokens(s.Context, s.smartAccountAddress3, "latest")
			s.Require().NoError(err)
			s.Require().Len(tokens, 2)
			s.Equal(types.NewValueFromUint64(10), tokens[*token1.id])
		})
	})

	///////////////////////////////////////////////////////////////////////////
	// Second part of testing: tests through TokensTest.sol

	tokenTest1 := CreateTokenId(&s.testAddress1_0)
	tokenTest2 := CreateTokenId(&s.testAddress1_1)

	s.Run("Create tokens for test addresses", func() {
		s.createTokenForTestContract(tokenTest1, types.NewValueFromUint64(1_000_000), "testToken1")
		s.createTokenForTestContract(tokenTest2, types.NewValueFromUint64(2_000_000), "testToken2")
	})

	s.Run("Call testCallWithTokensSync of testAddress1_0", func() {
		data, err := s.abiTest.Pack("testCallWithTokensSync", s.testAddress1_1,
			[]types.TokenBalance{{Token: *tokenTest1.id, Balance: types.NewValueFromUint64(5000)}})
		s.Require().NoError(err)

		hash, err := s.Client.SendExternalTransaction(
			s.Context, data, s.testAddress1_0, nil, types.NewFeePackFromGas(100_000))
		s.Require().NoError(err)
		receipt := s.WaitForReceipt(hash)
		s.Require().True(receipt.Success)

		s.Run("Check token is debited from testAddress1_0", func() {
			tokens, err := s.Client.GetTokens(s.Context, s.testAddress1_0, "latest")
			s.Require().NoError(err)
			s.Equal(types.NewValueFromUint64(1_000_000-5000), tokens[*tokenTest1.id])

			// Check balance via `Nil.tokenBalance` Solidity method
			data, err := s.abiTest.Pack(
				"checkTokenBalance", types.EmptyAddress, tokenTest1.id, big.NewInt(1_000_000-5000))
			s.Require().NoError(err)
			receipt := s.SendExternalTransactionNoCheck(data, s.testAddress1_0)
			s.Require().True(receipt.Success)
		})

		s.Run("Check token is credited to testAddress1_1", func() {
			tokens, err := s.Client.GetTokens(s.Context, s.testAddress1_1, "latest")
			s.Require().NoError(err)
			s.Equal(types.NewValueFromUint64(5000), tokens[*tokenTest1.id])
		})
	})

	invalidId := types.TokenId(types.HexToAddress("0x1234"))

	s.Run("Try to call with non-existent token", func() {
		data, err := s.abiTest.Pack("testCallWithTokensSync", s.testAddress1_1,
			[]types.TokenBalance{
				{Token: *tokenTest1.id, Balance: types.NewValueFromUint64(5000)},
				{Token: invalidId, Balance: types.NewValueFromUint64(1)},
			})
		s.Require().NoError(err)

		hash, err := s.Client.SendExternalTransaction(
			s.Context, data, s.testAddress1_0, nil, types.NewFeePackFromGas(100_000))
		s.Require().NoError(err)
		receipt := s.WaitForReceipt(hash)
		s.Require().False(receipt.Success)

		s.Run("Check token of testAddress1_0", func() {
			tokens, err := s.Client.GetTokens(s.Context, s.testAddress1_0, "latest")
			s.Require().NoError(err)
			s.Equal(types.NewValueFromUint64(1_000_000-5000), tokens[*tokenTest1.id])
		})

		s.Run("Check token of testAddress1_1", func() {
			tokens, err := s.Client.GetTokens(s.Context, s.testAddress1_1, "latest")
			s.Require().NoError(err)
			s.Equal(types.NewValueFromUint64(5000), tokens[*tokenTest1.id])
		})
	})

	s.Run("Call testCallWithTokensAsync of testAddress1_0", func() {
		data, err := s.abiTest.Pack("testCallWithTokensAsync", s.testAddress1_1,
			[]types.TokenBalance{{Token: *tokenTest1.id, Balance: types.NewValueFromUint64(5000)}})
		s.Require().NoError(err)

		hash, err := s.Client.SendExternalTransaction(
			s.Context, data, s.testAddress1_0, nil, types.NewFeePackFromGas(100_000))
		s.Require().NoError(err)
		receipt := s.WaitForReceipt(hash)
		s.Require().True(receipt.Success)
		s.Require().Len(receipt.OutReceipts, 1)
		s.Require().True(receipt.OutReceipts[0].Success)

		s.Run("Check token is debited from testAddress1_0", func() {
			tokens, err := s.Client.GetTokens(s.Context, s.testAddress1_0, "latest")
			s.Require().NoError(err)
			s.Equal(types.NewValueFromUint64(1_000_000-5000-5000), tokens[*tokenTest1.id])
		})

		s.Run("Check token is credited to testAddress1_1", func() {
			tokens, err := s.Client.GetTokens(s.Context, s.testAddress1_1, "latest")
			s.Require().NoError(err)
			s.Equal(types.NewValueFromUint64(5000+5000), tokens[*tokenTest1.id])
		})
	})

	s.Run("Try to call with non-existent token", func() {
		data, err := s.abiTest.Pack("testCallWithTokensAsync", s.testAddress1_1,
			[]types.TokenBalance{
				{Token: *tokenTest1.id, Balance: types.NewValueFromUint64(5000)},
				{Token: invalidId, Balance: types.NewValueFromUint64(1)},
			})
		s.Require().NoError(err)

		hash, err := s.Client.SendExternalTransaction(
			s.Context, data, s.testAddress1_0, nil, types.NewFeePackFromGas(100_000))
		s.Require().NoError(err)
		receipt := s.WaitForReceipt(hash)
		s.Require().False(receipt.Success)
		s.Require().Empty(receipt.OutReceipts)

		s.Run("Check token of testAddress1_0", func() {
			tokens, err := s.Client.GetTokens(s.Context, s.testAddress1_0, "latest")
			s.Require().NoError(err)
			s.Equal(types.NewValueFromUint64(1_000_000-5000-5000), tokens[*tokenTest1.id])
		})

		s.Run("Check token of testAddress1_1", func() {
			tokens, err := s.Client.GetTokens(s.Context, s.testAddress1_1, "latest")
			s.Require().NoError(err)
			s.Equal(types.NewValueFromUint64(5000+5000), tokens[*tokenTest1.id])
		})
	})

	amountTest1 := s.getTokenBalance(&s.testAddress1_0, tokenTest1)
	amountTest2 := s.getTokenBalance(&s.testAddress1_1, tokenTest1)

	s.Run("Call testSendTokensSync", func() {
		data, err := s.abiTest.Pack("testSendTokensSync", s.testAddress1_1, big.NewInt(5000), false)
		s.Require().NoError(err)

		hash, err := s.Client.SendExternalTransaction(
			s.Context, data, s.testAddress1_0, nil, types.NewFeePackFromGas(100_000))
		s.Require().NoError(err)
		receipt := s.WaitForReceipt(hash)
		s.Require().True(receipt.Success)
		s.Require().Empty(receipt.OutReceipts)
		s.Require().Empty(receipt.OutTransactions)

		s.Run("Check token was debited from testAddress1_0", func() {
			tokens, err := s.Client.GetTokens(s.Context, s.testAddress1_0, "latest")
			s.Require().NoError(err)
			s.Equal(amountTest1.Sub64(5000), tokens[*tokenTest1.id])
		})

		s.Run("Check token was credited to testAddress1_1", func() {
			tokens, err := s.Client.GetTokens(s.Context, s.testAddress1_1, "latest")
			s.Require().NoError(err)
			s.Equal(amountTest2.Add64(5000), tokens[*tokenTest1.id])
		})
	})

	s.Run("Call testSendTokensSync with fail flag", func() {
		data, err := s.abiTest.Pack("testSendTokensSync", s.testAddress1_1, big.NewInt(5000), true)
		s.Require().NoError(err)

		hash, err := s.Client.SendExternalTransaction(
			s.Context, data, s.testAddress1_0, nil, types.NewFeePackFromGas(100_000))
		s.Require().NoError(err)
		receipt := s.WaitForReceipt(hash)
		s.Require().False(receipt.Success)

		s.Run("Check token of testAddress1_0", func() {
			tokens, err := s.Client.GetTokens(s.Context, s.testAddress1_0, "latest")
			s.Require().NoError(err)
			s.Equal(amountTest1.Sub64(5000), tokens[*tokenTest1.id])
		})

		s.Run("Check token of testAddress1_1", func() {
			tokens, err := s.Client.GetTokens(s.Context, s.testAddress1_1, "latest")
			s.Require().NoError(err)
			s.Equal(amountTest2.Add64(5000), tokens[*tokenTest1.id])
		})
	})

	///////////////////////////////////////////////////////////////////////////
	// Call `testSendTokensSync` for address in different shard - should fail
	s.Run("Fail call testSendTokensSync for address in different shard", func() {
		data, err := s.abiTest.Pack("testSendTokensSync", s.smartAccountAddress3, big.NewInt(5000), false)
		s.Require().NoError(err)

		hash, err := s.Client.SendExternalTransaction(
			s.Context, data, s.testAddress1_0, nil, types.NewFeePackFromGas(100_000))
		s.Require().NoError(err)
		receipt := s.WaitForReceipt(hash)
		s.Require().False(receipt.Success)

		s.Run("Check token of testAddress1_0", func() {
			tokens, err := s.Client.GetTokens(s.Context, s.testAddress1_0, "latest")
			s.Require().NoError(err)
			s.Require().Equal(amountTest1.Sub64(5000), tokens[*tokenTest1.id])
		})
	})
}

func (s *SuiteMultiTokenRpc) TestTokenViaCall() {
	// Check that it's possible to call some function via eth_call
	// that works with tokens without crashes/errors.

	data := s.AbiPack(s.abiSmartAccount, "mintToken", big.NewInt(100))
	res, err := s.Client.Call(s.Context, &jsonrpc.CallArgs{
		To:   s.smartAccountAddress1,
		Data: (*hexutil.Bytes)(&data),
		Fee:  types.NewFeePackFromGas(100_000),
	}, "latest", nil)
	s.Require().NoError(err)
	s.Require().Empty(res.Error)
	s.Require().Positive(res.CoinsUsed.Uint64())
}

func (s *SuiteMultiTokenRpc) TestRemoveEmptyToken() {
	tokenSmartAccount1 := CreateTokenId(&s.smartAccountAddress1)

	amount := types.NewValueFromUint64(1_000_000)

	s.createTokenForTestContract(tokenSmartAccount1, amount, "smartAccount1")

	receipt := s.SendTransactionViaSmartAccountNoCheck(
		s.smartAccountAddress1,
		s.testAddress1_0,
		execution.MainPrivateKey,
		nil,
		types.NewFeePackFromGas(1_000_000),
		types.Value0,
		[]types.TokenBalance{{Token: *tokenSmartAccount1.id, Balance: amount}})
	s.Require().True(receipt.AllSuccess())

	tokens, err := s.Client.GetTokens(s.Context, s.smartAccountAddress1, "latest")
	s.Require().NoError(err)
	s.Require().Empty(tokens)

	tokens, err = s.Client.GetTokens(s.Context, s.testAddress1_0, "latest")
	s.Require().NoError(err)
	s.Require().Equal(amount, tokens[*tokenSmartAccount1.id])
}

func (s *SuiteMultiTokenRpc) TestBounce() {
	var (
		data    []byte
		tokens  types.TokensMap
		receipt *jsonrpc.RPCReceipt
		err     error
	)

	tokenSmartAccount1 := CreateTokenId(&s.smartAccountAddress1)

	s.createTokenForTestContract(tokenSmartAccount1, types.NewValueFromUint64(1_000_000), "smartAccount1")

	data, err = s.abiTest.Pack("receiveTokens", true)
	s.Require().NoError(err)

	receipt = s.SendTransactionViaSmartAccountNoCheck(
		s.smartAccountAddress1,
		s.testAddress1_0,
		execution.MainPrivateKey,
		data,
		types.FeePack{},
		types.NewValueFromUint64(2_000_000),
		[]types.TokenBalance{{Token: *tokenSmartAccount1.id, Balance: types.NewValueFromUint64(100)}})
	s.Require().True(receipt.Success)
	s.Require().Len(receipt.OutReceipts, 1)
	s.Require().False(receipt.OutReceipts[0].Success)

	// Check that nothing credited to a destination account
	tokens, err = s.Client.GetTokens(s.Context, s.testAddress1_0, "latest")
	s.Require().NoError(err)
	s.Require().Empty(tokens)

	// Check that token wasn't changed
	tokens, err = s.Client.GetTokens(s.Context, s.smartAccountAddress1, "latest")
	s.Require().NoError(err)
	s.Require().Equal(types.NewValueFromUint64(1_000_000), tokens[*tokenSmartAccount1.id])
}

func (s *SuiteMultiTokenRpc) TestIncomingBalance() {
	var (
		data    []byte
		receipt *jsonrpc.RPCReceipt
		err     error
	)

	tokenSmartAccount1 := CreateTokenId(&s.smartAccountAddress1)

	checkBalance := func(txnTokens *big.Int, accTokens *big.Int, receipt *jsonrpc.RPCReceipt) {
		a, err := s.abiTest.Events["tokenTxnBalance"].Inputs.Unpack(receipt.Logs[0].Data)
		s.Require().NoError(err)
		res, ok := a[0].(*big.Int)
		s.Require().True(ok)
		s.Require().Equal(*txnTokens, *res)

		a, err = s.abiTest.Events["tokenBalance"].Inputs.Unpack(receipt.Logs[1].Data)
		s.Require().NoError(err)
		res, ok = a[0].(*big.Int)
		s.Require().True(ok)
		s.Require().Equal(*accTokens, *res)
	}

	s.createTokenForTestContract(tokenSmartAccount1, types.NewValueFromUint64(1_000_000), "smartAccount1")

	data, err = s.abiTest.Pack("checkIncomingToken", *tokenSmartAccount1.id)
	s.Require().NoError(err)

	receipt = s.SendTransactionViaSmartAccountNoCheck(
		s.smartAccountAddress1,
		s.testAddress1_0,
		execution.MainPrivateKey,
		data,
		types.FeePack{},
		types.NewValueFromUint64(2_000_000),
		[]types.TokenBalance{{Token: *tokenSmartAccount1.id, Balance: types.NewValueFromUint64(100)}})
	s.Require().True(receipt.AllSuccess())

	checkBalance(big.NewInt(100), big.NewInt(100), receipt.OutReceipts[0])

	receipt = s.SendTransactionViaSmartAccountNoCheck(
		s.smartAccountAddress1,
		s.testAddress1_0,
		execution.MainPrivateKey,
		data,
		types.FeePack{}, types.NewValueFromUint64(2_000_000),
		[]types.TokenBalance{{Token: *tokenSmartAccount1.id, Balance: types.NewValueFromUint64(20_000)}})
	s.Require().True(receipt.AllSuccess())

	checkBalance(big.NewInt(20_000), big.NewInt(20_100), receipt.OutReceipts[0])
}

// NameTokensTestNoExternalAccess contract has no external access to token
func (s *SuiteMultiTokenRpc) TestNoExternalAccess() {
	abiTest, err := contracts.GetAbi(contracts.NameTokensTestNoExternalAccess)
	s.Require().NoError(err)

	token := CreateTokenId(&s.testAddressNoAccess)

	data := s.AbiPack(abiTest, "setTokenName", "TOKEN")
	receipt := s.SendExternalTransactionNoCheck(data, *token.address)
	s.Require().False(receipt.Success)
	s.Require().Equal("ExecutionReverted", receipt.Status)

	data = s.AbiPack(abiTest, "mintToken", big.NewInt(100_000))
	receipt = s.SendExternalTransactionNoCheck(data, *token.address)
	s.Require().False(receipt.Success)
	s.Require().Equal("ExecutionReverted", receipt.Status)

	data = s.AbiPack(abiTest, "sendToken", s.testAddress1_1, *token.id, big.NewInt(100_000))
	receipt = s.SendExternalTransactionNoCheck(data, *token.address)
	s.Require().False(receipt.Success)
	s.Require().Equal("ExecutionReverted", receipt.Status)
}

func (s *SuiteMultiTokenRpc) getTokenBalance(address *types.Address, token *TokenId) types.Value {
	s.T().Helper()

	tokens, err := s.Client.GetTokens(s.Context, *address, "latest")
	s.Require().NoError(err)
	return tokens[*token.id]
}

func (s *SuiteMultiTokenRpc) createTokenForTestContract(token *TokenId, amount types.Value, name string) {
	s.T().Helper()

	data := s.AbiPack(s.abiTest, "setTokenName", name)
	receipt := s.SendExternalTransactionNoCheck(data, *token.address)
	s.Require().True(receipt.Success)

	data = s.AbiPack(s.abiTest, "mintToken", amount.ToBig())
	receipt = s.SendExternalTransactionNoCheck(data, *token.address)
	s.Require().True(receipt.Success)

	// Check token is created and balance is correct
	tokens, err := s.Client.GetTokens(s.Context, *token.address, "latest")
	s.Require().NoError(err)
	s.Require().GreaterOrEqual(len(tokens), 1)
	s.Equal(amount, tokens[*token.id])

	// Check via getOwnTokenBalance method
	data = s.AbiPack(s.abiTest, "getOwnTokenBalance")
	data = s.CallGetter(*token.address, data, "latest", nil)
	results := s.AbiUnpack(s.abiTest, "getOwnTokenBalance", data)
	res, ok := results[0].(*big.Int)
	s.Require().True(ok)
	s.Require().Equal(amount.ToBig(), res)
}

type TokenId struct {
	address *types.Address
	id      *types.TokenId
}

func CreateTokenId(address *types.Address) *TokenId {
	id := types.TokenId(*address)
	return &TokenId{
		address: address,
		id:      &id,
	}
}

func TestMultiToken(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteMultiTokenRpc))
}
