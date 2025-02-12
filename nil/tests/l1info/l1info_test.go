package main

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/NilFoundation/nil/nil/internal/config"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/nilservice"
	"github.com/NilFoundation/nil/nil/services/rollup"
	"github.com/NilFoundation/nil/nil/services/rpc"
	"github.com/NilFoundation/nil/nil/tests"
	l1types "github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/suite"
)

const numShards = 4

type SuiteL1Info struct {
	tests.RpcSuite
	l1Fetcher *rollup.L1BlockFetcherMock
}

func (s *SuiteL1Info) SetupSuite() {
	s.l1Fetcher = &rollup.L1BlockFetcherMock{}

	block := &l1types.Header{
		Number:  big.NewInt(1),
		BaseFee: big.NewInt(1_000_000),
	}
	one := big.NewInt(1)
	lastBlockTm := time.Now()
	s.l1Fetcher.GetLastBlockInfoFunc = func(ctx context.Context) (*l1types.Header, error) {
		if time.Since(lastBlockTm) >= 5*time.Second {
			lastBlockTm = time.Now()
			block.Number = block.Number.Add(block.Number, one)
		}
		return block, nil
	}
}

func (s *SuiteL1Info) SetupTest() {
	s.Start(&nilservice.Config{
		NShards:              numShards,
		HttpUrl:              rpc.GetSockPath(s.T()),
		CollatorTickPeriodMs: 300,
		RunMode:              nilservice.CollatorsOnlyRunMode,
		L1Fetcher:            s.l1Fetcher,
	})
}

func (s *SuiteL1Info) TearDownTest() {
	s.Cancel()
}

func (s *SuiteL1Info) TestL1BlockUpdated() {
	s.Require().Eventually(func() bool {
		cfg := s.readConfig()
		return cfg.Number != 0
	}, 2*time.Second, 500*time.Millisecond)

	for i := range numShards {
		block, err := s.Client.GetBlock(s.Context, types.ShardId(i), "latest", false)
		s.Require().NoError(err)
		s.Require().NotEqual(0, block.L1Number)
	}
}

func (s *SuiteL1Info) readConfig() *config.ParamL1BlockInfo {
	s.T().Helper()

	roTx, err := s.Db.CreateRoTx(s.Context)
	s.Require().NoError(err)
	defer roTx.Rollback()

	cfgAccessor, err := config.NewConfigReader(roTx, nil)
	s.Require().NoError(err)
	cfg, err := config.GetParamL1Block(cfgAccessor)
	s.Require().NoError(err)
	return cfg
}

func TestL1Info(t *testing.T) {
	t.Parallel()

	suite.Run(t, &SuiteL1Info{})
}
