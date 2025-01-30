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
	client        *faucet.Client
	builtinFaucet bool
}

func (s *FaucetRpc) SetupTest() {
	sockPath := rpc.GetSockPath(s.T())

	s.Start(&nilservice.Config{
		NShards: 5,
		HttpUrl: sockPath,
	})

	if s.builtinFaucet {
		s.client = faucet.NewClient(sockPath)
	} else {
		s.client, _ = tests.StartFaucetService(s.T(), s.Context, &s.Wg, s.Client)
	}
	time.Sleep(time.Second)
}

func (s *FaucetRpc) TearDownTest() {
	s.Cancel()
}

func (s *FaucetRpc) TestSendRawTransaction() {
	faucets := types.GetTokens()
	res, err := s.client.GetFaucets()
	s.Require().NoError(err)
	s.Require().Equal(faucets, res)
}

func (s *FaucetRpc) TestSendToken() {
	expectedTokens := types.TokensMap{
		types.TokenId(types.EthFaucetAddress.Bytes()):  types.NewValueFromUint64(111),
		types.TokenId(types.BtcFaucetAddress.Bytes()):  types.NewValueFromUint64(222),
		types.TokenId(types.UsdtFaucetAddress.Bytes()): types.NewValueFromUint64(333),
		types.TokenId(types.UsdcFaucetAddress.Bytes()): types.NewValueFromUint64(444),
	}

	for i, addr := range []types.Address{types.EthFaucetAddress, types.BtcFaucetAddress, types.UsdtFaucetAddress, types.UsdcFaucetAddress} {
		amount := types.NewValueFromUint64(111 * uint64(i+1))
		viaFaucet, err := s.client.TopUpViaFaucet(addr, types.MainSmartAccountAddress, amount)
		s.Require().NoError(err)

		receipt := s.WaitForReceipt(viaFaucet)
		s.Require().True(receipt.Success)
	}
	tokens, err := s.RpcSuite.Client.GetTokens(s.Context, types.MainSmartAccountAddress, "latest")
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
