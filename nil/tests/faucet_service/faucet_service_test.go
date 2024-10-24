package main

import (
	"context"
	"testing"
	"time"

	rpc_client "github.com/NilFoundation/nil/nil/client/rpc"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/faucet"
	"github.com/NilFoundation/nil/nil/services/nilservice"
	"github.com/NilFoundation/nil/nil/services/rpc"
	"github.com/NilFoundation/nil/nil/tests"
	"github.com/stretchr/testify/suite"
)

type FaucetRpc struct {
	tests.RpcSuite
	client       *faucet.Client
	cancelFaucet context.CancelFunc
}

func (s *FaucetRpc) SetupTest() {
	sockPath := rpc.GetSockPath(s.T())
	faucetSockPath := rpc.GetSockPath(s.T())

	s.Start(&nilservice.Config{
		NShards: 5,
		HttpUrl: sockPath,
	})

	s.client = faucet.NewClient(faucetSockPath)

	ctx, cancel := context.WithCancel(context.Background())
	s.cancelFaucet = cancel

	serviceFaucet, err := faucet.NewService(rpc_client.NewClient(sockPath, logging.NewLogger("faucet")))
	s.Require().NoError(err)

	runErrCh := make(chan error, 1)
	s.Wg.Add(1)
	go func() {
		if err := serviceFaucet.Run(ctx, faucetSockPath); err != nil {
			runErrCh <- err
		} else {
			runErrCh <- nil
		}
		s.Wg.Done()
	}()
	time.Sleep(time.Second)

	select {
	case err := <-runErrCh:
		s.Require().NoError(err, "serviceFaucet failed to start")
	default:
	}
}

func (s *FaucetRpc) TearDownTest() {
	s.cancelFaucet()
	s.Cancel()
}

func (s *FaucetRpc) TestSendRawTransaction() {
	faucets := map[string]types.Address{
		"BTC":  types.BtcFaucetAddress,
		"ETH":  types.EthFaucetAddress,
		"MZK":  types.FaucetAddress,
		"USDT": types.UsdtFaucetAddress,
	}
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

		receipt := s.WaitForReceipt(types.BaseShardId, viaFaucet)
		s.Require().True(receipt.Success)
	}
	currencies, err := s.RpcSuite.Client.GetCurrencies(types.MainWalletAddress, "latest")
	s.Require().NoError(err)
	s.Require().Equal(expectedCurrencies, currencies)
}

func TestFaucetRpc(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(FaucetRpc))
}
