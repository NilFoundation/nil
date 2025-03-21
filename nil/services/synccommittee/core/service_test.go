package core

import (
	"context"
	"errors"
	"math/big"
	"testing"
	"time"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/services/nilservice"
	rpctest "github.com/NilFoundation/nil/nil/services/rpc"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/metrics"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/rollupcontract"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/storage"
	"github.com/NilFoundation/nil/nil/tests"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/suite"
)

type SyncCommitteeTestSuite struct {
	tests.RpcSuite

	url           string
	nShards       uint32
	blockStorage  *storage.BlockStorage
	syncCommittee *SyncCommittee
	scDb          db.DB
}

func (s *SyncCommitteeTestSuite) SetupSuite() {
	s.nShards = 4

	s.url = rpctest.GetSockPath(s.T())

	// Setup nilservice
	nilserviceCfg := &nilservice.Config{
		NShards:              s.nShards,
		HttpUrl:              s.url,
		CollatorTickPeriodMs: 200,
		DisableConsensus:     true,
	}

	s.Start(nilserviceCfg)

	var err error
	s.scDb, err = db.NewBadgerDbInMemory()
	s.Require().NoError(err)

	s.syncCommittee = s.newService()

	syncCommitteeMetrics, err := metrics.NewSyncCommitteeMetrics()
	s.Require().NoError(err)
	s.blockStorage = storage.NewBlockStorage(
		s.scDb,
		storage.DefaultBlockStorageConfig(),
		clockwork.NewRealClock(),
		syncCommitteeMetrics,
		logging.NewLogger("sync_committee_srv_test"),
	)
}

func (s *SyncCommitteeTestSuite) TearDownSuite() {
	s.Cancel()
	s.scDb.Close()
}

func (s *SyncCommitteeTestSuite) SetupTest() {
	err := s.scDb.DropAll()
	s.Require().NoError(err)

	s.syncCommittee = s.newService()
}

func (s *SyncCommitteeTestSuite) newService() *SyncCommittee {
	s.T().Helper()

	cfg := NewDefaultConfig()
	cfg.RpcEndpoint = s.url
	ethClientMock := &rollupcontract.EthClientMock{
		ChainIDFunc: func(ctx context.Context) (*big.Int, error) {
			return big.NewInt(0), errors.New("Empty mocked call")
		},
	}
	syncCommittee, err := New(cfg, s.scDb, ethClientMock)
	s.Require().NoError(err)
	return syncCommittee
}

func (s *SyncCommitteeTestSuite) waitMainShardToProcess() {
	s.T().Helper()
	s.Require().Eventually(
		func() bool {
			latestFetched, err := s.blockStorage.GetLatestFetched(s.Context)
			if err != nil {
				return false
			}
			mainRef := latestFetched.TryGetMain()
			return mainRef != nil && mainRef.Number > 0
		},
		5*time.Second,
		100*time.Millisecond,
	)
}

func (s *SyncCommitteeTestSuite) TestProcessingLoop() {
	// Run processing loop for a short time
	ctx, cancel := context.WithTimeout(s.Context, 5*time.Second)
	defer cancel()

	errCh := make(chan error)
	go func() {
		errCh <- s.syncCommittee.Run(ctx)
	}()

	s.waitMainShardToProcess()

	cancel()
	s.Require().ErrorIs(<-errCh, context.Canceled)
}

func (s *SyncCommitteeTestSuite) TestRun() {
	ctx, cancel := context.WithTimeout(s.Context, 5*time.Second)
	defer cancel()

	errCh := make(chan error)
	go func() {
		errCh <- s.syncCommittee.Run(ctx)
	}()

	s.waitMainShardToProcess()

	cancel() // to avoid waiting without reason
	s.Require().ErrorIs(<-errCh, context.Canceled)
}

func TestSyncCommitteeTestSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SyncCommitteeTestSuite))
}
