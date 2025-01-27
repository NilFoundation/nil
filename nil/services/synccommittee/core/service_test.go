package core

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/collate"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/services/nilservice"
	rpctest "github.com/NilFoundation/nil/nil/services/rpc"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/metrics"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/rollupcontract"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/storage"
	"github.com/NilFoundation/nil/nil/tests"
	"github.com/stretchr/testify/suite"
)

type SyncCommitteeTestSuite struct {
	suite.Suite

	server        tests.RpcSuite
	nShards       uint32
	blockStorage  storage.BlockStorage
	syncCommittee *SyncCommittee
	ctx           context.Context
	scDb          db.DB
}

func (s *SyncCommitteeTestSuite) SetupSuite() {
	s.nShards = 4
	s.ctx = context.Background()

	url := rpctest.GetSockPath(s.T())

	// Setup nilservice
	nilserviceCfg := &nilservice.Config{
		NShards:              s.nShards,
		HttpUrl:              url,
		Topology:             collate.TrivialShardTopologyId,
		CollatorTickPeriodMs: 100,
		GasBasePrice:         10,
	}

	nilContext, nilCancel := context.WithCancel(s.ctx)
	s.server.SetT(s.T())
	s.server.Context = nilContext
	s.server.CtxCancel = nilCancel
	s.server.Start(nilserviceCfg)

	cfg := NewDefaultConfig()
	cfg.RpcEndpoint = url

	var err error
	s.scDb, err = db.NewBadgerDbInMemory()
	s.Require().NoError(err)
	ethClientMock := &rollupcontract.EthClientMock{ChainIDFunc: func(ctx context.Context) (*big.Int, error) { return big.NewInt(0), nil }}
	s.syncCommittee, err = New(cfg, s.scDb, ethClientMock)
	s.Require().NoError(err)
	syncCommitteeMetrics, err := metrics.NewSyncCommitteeMetrics()
	s.Require().NoError(err)
	s.blockStorage = storage.NewBlockStorage(s.scDb, common.NewTimer(), syncCommitteeMetrics, logging.NewLogger("sync_committee_srv_test"))
	s.Require().NoError(err)
}

func (s *SyncCommitteeTestSuite) TearDownSuite() {
	s.server.Cancel()
	s.scDb.Close()
}

func (s *SyncCommitteeTestSuite) SetupTest() {
	err := s.scDb.DropAll()
	s.Require().NoError(err)
}

func (s *SyncCommitteeTestSuite) waitMainShardToProcess() {
	s.T().Helper()
	s.Require().Eventually(
		func() bool {
			lastFetched, err := s.blockStorage.TryGetLatestFetched(s.ctx)
			return err == nil && lastFetched != nil && lastFetched.Number > 0
		},
		5*time.Second,
		100*time.Millisecond,
	)
}

func (s *SyncCommitteeTestSuite) TestProcessingLoop() {
	// Run processing loop for a short time
	ctx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
	defer cancel()

	errCh := make(chan error)
	go func() {
		errCh <- s.syncCommittee.Run(ctx)
	}()

	s.waitMainShardToProcess()

	cancel() // to avoid waiting without reason
	s.Require().NoError(<-errCh)
}

func (s *SyncCommitteeTestSuite) TestRun() {
	ctx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
	defer cancel()

	errCh := make(chan error)
	go func() {
		errCh <- s.syncCommittee.Run(ctx)
	}()

	s.waitMainShardToProcess()

	cancel() // to avoid waiting without reason
	s.Require().NoError(<-errCh)
}

func TestSyncCommitteeTestSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SyncCommitteeTestSuite))
}
