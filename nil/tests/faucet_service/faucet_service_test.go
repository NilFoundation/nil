package main

import (
	"testing"
	"time"

	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/faucet"
	"github.com/NilFoundation/nil/nil/services/nilservice"
	"github.com/NilFoundation/nil/nil/services/rpc"
	"github.com/NilFoundation/nil/nil/tests"
	"github.com/stretchr/testify/suite"
)

type FaucetRpc struct {
	tests.RpcSuite
	faucetClient  *faucet.Client
	builtinFaucet bool
}

func (s *FaucetRpc) SetupSuite() {
	sockPath := rpc.GetSockPath(s.T())

	s.Start(&nilservice.Config{
		NShards: 5,
		HttpUrl: sockPath,
	})

	if s.builtinFaucet {
		s.faucetClient = faucet.NewClient(sockPath)
	} else {
		s.faucetClient, _ = tests.StartFaucetService(s.Context, s.T(), &s.Wg, s.Client)
	}
	time.Sleep(time.Second)
}

func (s *FaucetRpc) TearDownSuite() {
	s.Cancel()
}

func (s *FaucetRpc) topUpAndWait(faucetAddr, addr types.Address, amount types.Value) {
	s.T().Helper()

	hash, err := s.faucetClient.TopUpViaFaucet(faucetAddr, addr, amount)
	s.Require().NoError(err)

	receipt := s.WaitForReceipt(hash)
	s.Require().True(receipt.AllSuccess())
}

func (s *FaucetRpc) TestDefaultToken() {
	s.Run("GetFaucets", func() {
		faucets := types.GetTokens()
		s.Require().Equal(types.FaucetAddress, faucets["NIL"])

		res, err := s.faucetClient.GetFaucets()
		s.Require().NoError(err)
		s.Require().Equal(faucets, res)
	})

	s.Run("TopUpViaFaucet", func() {
		addr := types.GenerateRandomAddress(types.BaseShardId)
		total := types.NewZeroValue()
		amount := types.NewValueFromUint64(100)
		for range 5 {
			amount = amount.Mul64(10)
			s.topUpAndWait(types.FaucetAddress, addr, amount)

			total = total.Add(amount)
			b, err := s.Client.GetBalance(s.Context, addr, "latest")
			s.Require().NoError(err)
			s.Require().Equal(total, b)
		}
	})

	if !s.builtinFaucet {
		s.Run("TopUpViaFaucet from another service", func() {
			otherClient := faucet.NewClient(s.Config.HttpUrl)
			viaFaucet, err := otherClient.TopUpViaFaucet(types.FaucetAddress,
				types.GenerateRandomAddress(types.BaseShardId), types.NewValueFromUint64(100))
			s.Require().NoError(err)

			receipt := s.WaitForReceipt(viaFaucet)
			s.Require().True(receipt.Success)
		})

		s.Run("TopUpViaFaucet works on the initial service", func() {
			s.topUpAndWait(types.FaucetAddress,
				types.GenerateRandomAddress(types.BaseShardId), types.NewValueFromUint64(100))
		})
	}
}

func (s *FaucetRpc) TestSendToken() {
	expectedTokens := types.TokensMap{}

	wallet := types.MainSmartAccountAddress
	for i, addr := range []types.Address{
		types.EthFaucetAddress,
		types.BtcFaucetAddress,
		types.UsdtFaucetAddress,
		types.UsdcFaucetAddress,
	} {
		amount := types.NewValueFromUint64(111 * uint64(i+1))
		expectedTokens[types.TokenId(addr.Bytes())] = amount
		s.topUpAndWait(addr, wallet, amount)
	}
	tokens, err := s.Client.GetTokens(s.Context, wallet, "latest")
	s.Require().NoError(err)
	s.Require().Equal(expectedTokens, tokens)
}

func TestFaucetRpc(t *testing.T) {
	t.Parallel()

	suite.Run(t, &FaucetRpc{builtinFaucet: false})
}

func TestBuiltInFaucetRpc(t *testing.T) {
	t.Parallel()

	suite.Run(t, &FaucetRpc{builtinFaucet: true})
}
