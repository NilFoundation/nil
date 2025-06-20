package main

import (
	"crypto/ecdsa"
	"math/big"
	"testing"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/contracts"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/faucet"
	"github.com/NilFoundation/nil/nil/services/nilservice"
	"github.com/NilFoundation/nil/nil/services/rpc/transport"
	"github.com/NilFoundation/nil/nil/tests"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/suite"
)

type SuiteFaucet struct {
	tests.ShardedSuite
	faucetClient *faucet.Client
}

func (s *SuiteFaucet) SetupSuite() {
	s.Start(&nilservice.Config{
		NShards:              3,
		CollatorTickPeriodMs: 200,
	}, 10225)

	s.DefaultClient, _ = s.StartRPCNode(&tests.RpcNodeConfig{
		WithDhtBootstrapByValidators: true,
		ArchiveNodes:                 nil,
	})
	s.faucetClient, _ = tests.StartFaucetService(s.Context, s.T(), &s.Wg, s.DefaultClient)
}

func (s *SuiteFaucet) TearDownSuite() {
	s.Cancel()
}

func (s *SuiteFaucet) createSmartAccountViaFaucet(ownerPrivateKey *ecdsa.PrivateKey, value int64) types.Address {
	s.T().Helper()

	ownerPublicKey := crypto.FromECDSAPub(&ownerPrivateKey.PublicKey)

	salt := uint256.NewInt(123).Bytes32()
	callData, err := contracts.NewCallData(
		contracts.NameFaucet, "createSmartAccount", ownerPublicKey, salt, big.NewInt(value))
	s.Require().NoError(err)

	resHash, err := s.DefaultClient.SendExternalTransaction(
		s.Context, callData, types.FaucetAddress, nil, types.FeePack{})
	s.Require().NoError(err)

	res := s.WaitIncludedInMain(resHash)
	s.Require().True(res.AllSuccess())

	// Checking whether the address generated by CREATE2 matches the expected one
	smartAccountCode := contracts.PrepareDefaultSmartAccountForOwnerCode(ownerPublicKey)
	smartAccountAddr := types.CreateAddressForCreate2(types.FaucetAddress, smartAccountCode, salt)
	s.Require().Equal(common.LeftPadBytes(smartAccountAddr.Bytes(), 32), []byte(res.Logs[0].Data[:]))
	return smartAccountAddr
}

func (s *SuiteFaucet) TestCreateSmartAccountViaFaucet() {
	userPrivateKey, err := crypto.GenerateKey()
	s.Require().NoError(err)

	var value int64 = 100000
	smartAccountAddress := s.createSmartAccountViaFaucet(userPrivateKey, value)

	blockNumber := transport.LatestBlockNumber
	balance, err := s.DefaultClient.GetBalance(s.Context, smartAccountAddress,
		transport.BlockNumberOrHash{BlockNumber: &blockNumber})
	s.Require().NoError(err)

	s.Require().NoError(err)
	s.Require().Equal(uint64(value), balance.Uint64())
}

func (s *SuiteFaucet) TestDeployContractViaFaucet() {
	userPrivateKey, err := crypto.GenerateKey()
	s.Require().NoError(err)
	userPublicKey := crypto.FromECDSAPub(&userPrivateKey.PublicKey)

	value := types.GasToValue(123_456_789)
	smartAccountCode := contracts.PrepareDefaultSmartAccountForOwnerCode(userPublicKey)

	code := types.BuildDeployPayload(smartAccountCode, common.EmptyHash)
	smartAccountAddr := types.CreateAddress(types.FaucetAddress.ShardId(), code)
	txnHash, err := s.faucetClient.TopUpViaFaucet(s.Context, types.FaucetAddress, smartAccountAddr, value)
	s.Require().NoError(err)
	receipt := s.WaitIncludedInMain(txnHash)
	s.Require().True(receipt.AllSuccess())

	txnHash, receiptContractAddress, err := s.DefaultClient.DeployExternal(
		s.Context, smartAccountAddr.ShardId(), code, types.NewFeePackFromGas(10_000_000))
	s.Require().NoError(err)
	s.Require().Equal(smartAccountAddr, receiptContractAddress)
	receipt = s.WaitIncludedInMain(txnHash)
	s.Require().True(receipt.AllSuccess())

	balance, err := s.DefaultClient.GetBalance(s.Context, smartAccountAddr, "latest")
	s.Require().NoError(err)
	s.Require().Less(balance.Uint64(), value.Uint64())
	s.Require().NotZero(balance.Uint64())
	logging.GlobalLogger.Info().Msgf("Spent %s nil", value.Sub(balance))
}

func (s *SuiteFaucet) TestTopUpViaFaucet() {
	pk, err := crypto.GenerateKey()
	s.Require().NoError(err)
	pubKey := crypto.FromECDSAPub(&pk.PublicKey)
	smartAccountCode := contracts.PrepareDefaultSmartAccountForOwnerCode(pubKey)

	address, receipt := s.DeployContractViaMainSmartAccount(
		types.BaseShardId,
		types.BuildDeployPayload(smartAccountCode, common.EmptyHash),
		types.Value{})
	receipt = s.WaitIncludedInMain(receipt.TxnHash)
	s.Require().NotNil(receipt)
	s.Require().True(receipt.AllSuccess())

	testTopUp := func(initialValue, value, expectedValue uint64, delta float64) {
		balance, err := s.DefaultClient.GetBalance(s.Context, address, transport.LatestBlockNumber)
		s.Require().NoError(err)
		s.Require().Equal(initialValue, balance.Uint64())

		code, err := s.DefaultClient.GetCode(s.Context, address, transport.LatestBlockNumber)
		s.Require().NoError(err)
		s.Require().NotEmpty(code)

		mshHash, err := s.faucetClient.TopUpViaFaucet(
			s.Context, types.FaucetAddress, address, types.NewValueFromUint64(value))
		s.Require().NoError(err)
		receipt = s.WaitIncludedInMain(mshHash)
		s.Require().NotNil(receipt)
		s.Require().True(receipt.AllSuccess())

		balance, err = s.DefaultClient.GetBalance(s.Context, address, transport.LatestBlockNumber)
		s.Require().NoError(err)
		s.Require().InDelta(expectedValue, balance.Uint64(), delta)
	}

	var value1 uint64 = 5 * 1_000_000_000_000_000
	balance1 := value1
	s.Run("Top up for the first time without exceeding the limit", func() {
		testTopUp(0, value1, balance1, 0)
	})

	var value2 uint64 = 4 * 1_000_000_000_000_000
	balance2 := balance1 + value2
	s.Run("Top up for the second time without exceeding the limit", func() {
		testTopUp(balance1, value2, balance2, 0)
	})

	// this test is quite flaky, cause it checks
	// functionality that depends on block generation speed
	var value3 uint64 = 5 * 1_000_000_000_000_000
	var balance3 uint64 = 10_000_000_000_000_000
	s.Run("Top up over limit", func() {
		testTopUp(balance2, value3, balance3, float64(balance3)*0.2)
	})
}

func (s *SuiteFaucet) TestTopUpTokenViaFaucet() {
	pk, err := crypto.GenerateKey()
	s.Require().NoError(err)
	pubKey := crypto.FromECDSAPub(&pk.PublicKey)
	smartAccountCode := contracts.PrepareDefaultSmartAccountForOwnerCode(pubKey)

	address, receipt := s.DeployContractViaMainSmartAccount(
		types.BaseShardId,
		types.BuildDeployPayload(smartAccountCode, common.EmptyHash),
		types.Value{})
	receipt = s.WaitIncludedInMain(receipt.TxnHash)
	s.Require().NotNil(receipt)
	s.Require().True(receipt.AllSuccess())

	value := types.NewValueFromUint64(1000)
	faucetsAddr := []types.Address{
		types.EthFaucetAddress,
		types.UsdtFaucetAddress,
		types.BtcFaucetAddress,
		types.UsdcFaucetAddress,
	}
	for _, faucet := range faucetsAddr {
		mshHash, err := s.faucetClient.TopUpViaFaucet(s.Context, faucet, address, value)
		s.Require().NoError(err)
		receipt = s.WaitIncludedInMain(mshHash)
		s.Require().NotNil(receipt)
		s.Require().True(receipt.AllSuccess())
	}
	tokens, err := s.DefaultClient.GetTokens(s.Context, address, transport.LatestBlockNumber)
	s.Require().NoError(err)
	s.Require().Len(tokens, 4)

	debugContract, err := s.DefaultClient.GetDebugContract(s.Context, address, "latest")
	s.Require().NoError(err)
	s.Require().Len(debugContract.Tokens, 4)

	for _, faucet := range faucetsAddr {
		curValue, ok := tokens[types.TokenId(faucet)]
		s.Require().True(ok)
		s.Require().Equal(value.Uint64(), curValue.Uint64())

		curValue, ok = debugContract.Tokens[types.TokenId(faucet)]
		s.Require().True(ok)
		s.Require().Equal(value.Uint64(), curValue.Uint64())
	}
}

func TestSuiteFaucet(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteFaucet))
}
