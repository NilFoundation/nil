package main

import (
	"testing"
	"time"

	"github.com/NilFoundation/nil/nil/internal/config"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/nilservice"
	"github.com/NilFoundation/nil/nil/services/rpc"
	"github.com/NilFoundation/nil/nil/tests"
	"github.com/stretchr/testify/suite"
)

const numShards = 4

type SuiteL1Info struct {
	tests.RpcSuite
}

func (s *SuiteL1Info) SetupSuite() {
}

func (s *SuiteL1Info) SetupTest() {
	s.Start(&nilservice.Config{
		NShards:              numShards,
		HttpUrl:              rpc.GetSockPath(s.T()),
		CollatorTickPeriodMs: 300,
		RunMode:              nilservice.CollatorsOnlyRunMode,
		L1Fetcher:            tests.CreateMockL1Fetcher(s.T(), 5*time.Second),
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
