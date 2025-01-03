package core

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/NilFoundation/nil/nil/client/rpc"
	"github.com/NilFoundation/nil/nil/internal/collate"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/nilservice"
	rpctest "github.com/NilFoundation/nil/nil/services/rpc"
	"github.com/NilFoundation/nil/nil/services/rpc/transport"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/rollupcontract"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/suite"
)

type SyncCommitteeTestSuite struct {
	suite.Suite

	nShards       uint32
	syncCommittee *SyncCommittee
	nilCancel     context.CancelFunc
	doneChan      chan interface{} // to track when nilservice has finished
	ctx           context.Context
	nilDb         db.DB
	scDb          db.DB
}

func (s *SyncCommitteeTestSuite) waitZerostrate(endpoint string) {
	s.T().Helper()
	client := rpc.NewClient(endpoint, zerolog.Nop())
	const (
		zeroStateWaitTimeout  = 5 * time.Second
		zeroStatePollInterval = time.Second
	)
	for i := range s.nShards {
		s.Require().Eventually(func() bool {
			block, err := client.GetBlock(s.ctx, types.ShardId(i), transport.BlockNumber(0), false)
			return err == nil && block != nil
		}, zeroStateWaitTimeout, zeroStatePollInterval)
	}
}

func (s *SyncCommitteeTestSuite) SetupSuite() {
	s.nShards = 4
	s.ctx = context.Background()

	url := rpctest.GetSockPath(s.T())

	var err error
	s.nilDb, err = db.NewBadgerDbInMemory()
	s.Require().NoError(err)

	// Setup nilservice
	nilserviceCfg := &nilservice.Config{
		NShards:              s.nShards,
		HttpUrl:              url,
		Topology:             collate.TrivialShardTopologyId,
		CollatorTickPeriodMs: 100,
		GasBasePrice:         10,
	}
	var nilContext context.Context
	nilContext, s.nilCancel = context.WithCancel(context.Background())
	s.doneChan = make(chan interface{})
	go func() {
		nilservice.Run(nilContext, nilserviceCfg, s.nilDb, nil)
		s.doneChan <- nil
	}()

	s.waitZerostrate(url)

	cfg := NewDefaultConfig()
	cfg.RpcEndpoint = url

	s.scDb, err = db.NewBadgerDbInMemory()
	s.Require().NoError(err)
	ethClientMock := &rollupcontract.EthClientMock{ChainIDFunc: func(ctx context.Context) (*big.Int, error) { return big.NewInt(0), nil }}
	s.syncCommittee, err = New(cfg, s.scDb, ethClientMock)
	s.Require().NoError(err)
}

func (s *SyncCommitteeTestSuite) TearDownSuite() {
	s.nilCancel()
	<-s.doneChan // Wait for nilservice to shutdown
	s.nilDb.Close()
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
			lastFetched, err := s.syncCommittee.aggregator.blockStorage.TryGetLatestFetched(s.ctx)
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
		errCh <- s.syncCommittee.aggregator.Run(ctx)
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
