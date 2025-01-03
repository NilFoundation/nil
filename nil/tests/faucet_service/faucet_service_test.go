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
	faucets := types.GetCurrencies()
	res, err := s.client.GetFaucets()
	s.Require().NoError(err)
	s.Require().Equal(faucets, res)
}

func (s *FaucetRpc) TestSendToken() {
	expectedCurrencies := types.CurrenciesMap{
		types.CurrencyId(types.EthFaucetAddress.Bytes()):  types.NewValueFromUint64(111),
		types.CurrencyId(types.BtcFaucetAddress.Bytes()):  types.NewValueFromUint64(222),
		types.CurrencyId(types.UsdtFaucetAddress.Bytes()): types.NewValueFromUint64(333),
	}

	for i, addr := range []types.Address{types.EthFaucetAddress, types.BtcFaucetAddress, types.UsdtFaucetAddress} {
		amount := types.NewValueFromUint64(111 * uint64(i+1))
		viaFaucet, err := s.client.TopUpViaFaucet(addr, types.MainWalletAddress, amount)
		s.Require().NoError(err)

		receipt := s.WaitForReceipt(viaFaucet)
		s.Require().True(receipt.Success)
	}
	currencies, err := s.RpcSuite.Client.GetCurrencies(s.Context, types.MainWalletAddress, "latest")
	s.Require().NoError(err)
	s.Require().Equal(expectedCurrencies, currencies)
}

func TestFaucetRpc(t *testing.T) {
	t.Parallel()

	suite.Run(t, &FaucetRpc{builtinFaucet: false})
}

func TestBuiltInFaucetRpc(t *testing.T) {
	t.Parallel()

	suite.Run(t, &FaucetRpc{builtinFaucet: true})
}
