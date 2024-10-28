package main

import (
	"testing"
	"time"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/faucet"
	"github.com/NilFoundation/nil/nil/services/nil_load_generator"
	"github.com/NilFoundation/nil/nil/services/nilservice"
	"github.com/NilFoundation/nil/nil/services/rpc"
	"github.com/NilFoundation/nil/nil/tests"
	"github.com/stretchr/testify/suite"
)

type NilLoadGeneratorRpc struct {
	tests.RpcSuite
	endpoint     string
	runErrCh     chan error
	faucetClient *faucet.Client
}

func (s *NilLoadGeneratorRpc) SetupTest() {
	sockPath := rpc.GetSockPath(s.T())
	nilLoadGeneratorSockPath := rpc.GetSockPath(s.T())
	s.endpoint = nilLoadGeneratorSockPath
	s.Start(&nilservice.Config{
		NShards:              4,
		HttpUrl:              sockPath,
		CollatorTickPeriodMs: 50,
	})

	var faucetEndpoint string
	s.faucetClient, faucetEndpoint = tests.StartFaucetService(s.T(), s.Context, &s.Wg, s.Client)
	time.Sleep(time.Second)

	s.runErrCh = make(chan error, 1)
	s.Wg.Add(1)
	go func() {
		defer s.Wg.Done()
		if err := nil_load_generator.Run(s.Context, nil_load_generator.Config{OwnEndpoint: nilLoadGeneratorSockPath, Endpoint: sockPath, FaucetEndpoint: faucetEndpoint, SwapPerIteration: 1},
			logging.NewLogger("test_nil_load_generator")); err != nil {
			s.runErrCh <- err
		} else {
			s.runErrCh <- nil
		}
	}()
	time.Sleep(time.Second)
}

func (s *NilLoadGeneratorRpc) TearDownTest() {
	s.Cancel()
}

func (s *NilLoadGeneratorRpc) TestWalletBalanceModification() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	testTimeout := time.After(15 * time.Second)

	client := nil_load_generator.NewClient(s.endpoint)

	var err error
	shardIdList, err := s.Client.GetShardIdList()
	s.Require().NoError(err)

	var resWallets []types.Address
	walletsBalance := make([]types.Value, len(shardIdList))

	s.Require().Eventually(func() bool {
		resWallets, err = client.GetWalletsAddr()
		s.Require().NoError(err)
		for i, addr := range resWallets {
			walletsBalance[i], err = s.Client.GetBalance(addr, "latest")
			s.Require().NoError(err)
		}
		return len(resWallets) != 0
	}, 20*time.Second, 100*time.Millisecond)

	for {
		select {
		case <-testTimeout:
			for i, addr := range resWallets {
				newBalance, err := s.Client.GetBalance(addr, "latest")
				s.Require().NoError(err)
				s.Require().Greater(walletsBalance[i].Uint64(), newBalance.Uint64())
			}
			return
		case <-ticker.C:
			res, err := client.GetHealthCheck()
			s.Require().NoError(err)
			s.Require().True(res)
		case err := <-s.runErrCh:
			if err != nil {
				s.Require().NoError(err)
			}
		}
	}
}

func TestNilLoadGeneratorRpcRpc(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(NilLoadGeneratorRpc))
}
