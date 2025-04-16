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
		CollatorTickPeriodMs: 30,
	})

	var faucetEndpoint string
	s.faucetClient, faucetEndpoint = tests.StartFaucetService(s.Context, s.T(), &s.Wg, s.Client)
	time.Sleep(time.Second)

	s.runErrCh = make(chan error, 1)
	s.Wg.Add(1)
	go func() {
		defer s.Wg.Done()
		if err := nil_load_generator.Run(
			s.Context,
			&nil_load_generator.Config{
				OwnEndpoint:        nilLoadGeneratorSockPath,
				Endpoint:           sockPath,
				FaucetEndpoint:     faucetEndpoint,
				SwapPerIteration:   1,
				RpcSwapLimit:       *types.NewUint256(10000000),
				MintTokenAmount0:   types.NewValueFromUint64(3000000),
				MintTokenAmount1:   types.NewValueFromUint64(10000),
				ThresholdAmount:    types.NewValueFromUint64(3000000000),
				SwapAmount:         types.NewValueFromUint64(1000),
				WaitClusterStartup: 10 * time.Second,
			},
			logging.NewLogger("test_nil_load_generator")); err != nil {
			s.runErrCh <- err
		} else {
			s.runErrCh <- nil
		}
	}()
	time.Sleep(3 * time.Second)
}

func (s *NilLoadGeneratorRpc) TearDownTest() {
	s.Cancel()
}

func (s *NilLoadGeneratorRpc) TestSmartAccountBalanceModification() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	testTimeout := time.After(15 * time.Second)

	client := nil_load_generator.NewClient(s.endpoint)

	var err error
	shardIdList, err := s.Client.GetShardIdList(s.Context)
	s.Require().NoError(err)

	var resSmartAccounts []types.Address
	smartAccountsBalance := make([]types.Value, len(shardIdList))

	s.Require().Eventually(func() bool {
		resSmartAccounts, err = client.GetSmartAccountsAddr(s.Context)
		s.Require().NoError(err)
		for i, addr := range resSmartAccounts {
			smartAccountsBalance[i], err = s.Client.GetBalance(s.Context, addr, "latest")
			s.Require().NoError(err)
		}
		return len(resSmartAccounts) != 0
	}, 60*time.Second, 100*time.Millisecond)

	for i, addr := range resSmartAccounts {
		s.Require().NotZero(smartAccountsBalance[i].Uint64(),
			"Zero balance for smart account %d, addr %s", i, addr)
	}

	for {
		select {
		case <-testTimeout:
			return
		case <-ticker.C:
			res, err := client.GetHealthCheck(s.Context)
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
