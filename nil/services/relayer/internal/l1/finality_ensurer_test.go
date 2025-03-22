package l1

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/services/relayer/internal/l2"
	ethcommon "github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/jonboulle/clockwork"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/suite"
)

type eventListenerStub struct {
	emitter chan struct{}
}

func newEventListenerStub() *eventListenerStub {
	return &eventListenerStub{
		emitter: make(chan struct{}),
	}
}

// Can be used by reading routine to look for updates without further delay
func (els *eventListenerStub) EventReceived() <-chan struct{} {
	return els.emitter
}

func (els *eventListenerStub) emit() {
	els.emitter <- struct{}{}
}

func (els *eventListenerStub) waitForEnsurerLoop() {
	els.emit()

	// channel is not buffered so by the moment
	// the emitter is able to exit from the second emit call
	// testing finality ensurer must run at least one full loop iteration
	els.emit()
}

type FinalityEnsurerTestSuite struct {
	suite.Suite

	// high level dependencies
	database  db.DB
	l1Storage *EventStorage
	l2Storage *l2.EventStorage
	logger    logging.Logger

	// testing entity
	ensurer *FinalityEnsurer

	// mocks
	ethClientMock     *EthClientMock
	mockLatestBlock   *uint64
	mockBlockHdrByNum map[uint64]ethtypes.Header

	clockMock         *clockwork.FakeClock
	eventListenerStub *eventListenerStub

	// testing lifecycle stuff
	ctx            context.Context
	canceler       context.CancelFunc
	ensurerStopped chan struct{}
}

func TestFinalityEnsurer(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(FinalityEnsurerTestSuite))
}

func (s *FinalityEnsurerTestSuite) SetupTest() {
	var err error

	s.ctx, s.canceler = context.WithCancel(context.Background())
	s.logger = logging.NewFromZerolog(zerolog.New(zerolog.NewConsoleWriter()))

	s.database, err = db.NewBadgerDbInMemory()
	s.Require().NoError(err, "failed to initialize database")

	s.clockMock = clockwork.NewFakeClock()

	s.ethClientMock = &EthClientMock{}
	s.mockBlockHdrByNum = make(map[uint64]ethtypes.Header)
	s.ethClientMock.HeaderByNumberFunc = func(ctx context.Context, number *big.Int) (*ethtypes.Header, error) {
		s.logger.Warn().Int64("req_blk", number.Int64()).Msg("requested block header from L1")
		if rpc.BlockNumber(number.Int64()) == rpc.FinalizedBlockNumber && s.mockLatestBlock != nil {
			return &ethtypes.Header{
				Number: big.NewInt(int64(*s.mockLatestBlock)),
			}, nil
		} else {
			if hdr, ok := s.mockBlockHdrByNum[number.Uint64()]; ok {
				return &hdr, nil
			}
		}
		return nil, nil
	}

	s.l1Storage, err = NewEventStorage(s.ctx, s.database, s.clockMock, nil, s.logger)
	s.Require().NoError(err, "failed to initialize L1 storage")

	s.l2Storage = l2.NewEventStorage(s.ctx, s.database, s.clockMock, nil, s.logger)

	cfg := DefaultFinalityEnsurerConfig()
	cfg.EventEmitterCapacity = 100

	s.eventListenerStub = newEventListenerStub()

	s.ensurer, err = NewFinalityEnsurer(
		cfg,
		s.ethClientMock,
		s.clockMock,
		s.logger,
		s.l1Storage,
		s.l2Storage,
		s.eventListenerStub,
	)
	s.Require().NoError(err)

	started := make(chan struct{})
	go func() {
		s.ensurerStopped = make(chan struct{})
		defer close(s.ensurerStopped)
		err := s.ensurer.Run(s.ctx, started)
		if err != nil {
			s.ErrorIs(err, context.Canceled)
		}
	}()

	<-started
}

func (s *FinalityEnsurerTestSuite) TearDownTest() {
	s.canceler()
	<-s.ensurerStopped
}

func (s *FinalityEnsurerTestSuite) setFinalizedBlockNumer(n uint64) {
	s.mockLatestBlock = &n
}

func (s *FinalityEnsurerTestSuite) setBlockHeaderOnL1(hdr ethtypes.Header) {
	s.mockBlockHdrByNum[hdr.Number.Uint64()] = hdr
}

func (s *FinalityEnsurerTestSuite) advanceFinalizedBlockNumberTo(n uint64) {
	s.T().Helper()

	s.setFinalizedBlockNumer(n)
	s.clockMock.Advance(time.Hour)

	err := common.WaitFor(
		s.ctx, time.Second, time.Millisecond,
		func(ctx context.Context) bool {
			if blk, ok := s.ensurer.getLatestFinalizedBlock(); ok && blk.BlockNumber == n {
				return true
			}
			return false
		},
	)
	s.Require().NoError(err, "ensurer fin block fetcher is idle")
}

func (s *FinalityEnsurerTestSuite) checkL2StorageContent(blockNumbers ...uint64) {
	s.T().Helper()

	set := make(map[uint64]bool)
	for _, n := range blockNumbers {
		set[n] = false
	}

	err := s.l2Storage.IterateEventsByBatch(s.ctx, 100, func(events []*l2.Event) error {
		for _, evt := range events {
			s.Require().False(set[evt.BlockNumber], "event from block %d is duplicated in l2 storage", evt.BlockNumber)
			set[evt.BlockNumber] = true
		}
		return nil
	})
	s.Require().NoError(err)

	for blockNumber, found := range set {
		s.True(found, "event from block number %d is not relayed", blockNumber)
	}
}

func (s *FinalityEnsurerTestSuite) TestFinalizedBlock() {
	const N = 1000

	l1Headers := []ethtypes.Header{
		{
			Number: big.NewInt(N - 1),
		},
		{
			Number: big.NewInt(N),
		},
		{
			Number: big.NewInt(N + 1),
		},
	}

	for i := range l1Headers {
		err := s.l1Storage.StoreEvent(s.ctx, &Event{
			Hash:        getMsgHash(msgSourceSubscription, i),
			BlockNumber: l1Headers[i].Number.Uint64(),
			BlockHash:   l1Headers[i].Hash(),
		})
		s.Require().NoError(err)
		s.setBlockHeaderOnL1(l1Headers[i])
	}

	s.eventListenerStub.waitForEnsurerLoop()

	err := s.l2Storage.IterateEventsByBatch(s.ctx, 100, func(events []*l2.Event) error {
		s.Fail("L2 event is not expected", "found %d events", len(events))
		return nil
	})
	s.Require().NoError(err)

	s.advanceFinalizedBlockNumberTo(N)
	s.eventListenerStub.waitForEnsurerLoop()
	s.checkL2StorageContent(N-1, N)

	s.advanceFinalizedBlockNumberTo(N + 100)
	s.eventListenerStub.waitForEnsurerLoop()
	s.checkL2StorageContent(N-1, N, N+1)

	err = s.l1Storage.IterateEventsByBatch(s.ctx, 100, func(events []*Event) error {
		s.Fail("some events unexpectedly left in L1 storage", "found %d events", len(events))
		return nil
	})
	s.Require().NoError(err)
}

func (s *FinalityEnsurerTestSuite) TestOrphanedBlock() {
	const N = 1000

	l1Headers := []ethtypes.Header{
		{
			Number: big.NewInt(N - 1),
		},
		{
			Number: big.NewInt(N),
		},
		{
			Number:     big.NewInt(N + 1),
			ParentHash: ethcommon.HexToHash("0xDEADBEEF"), // this one is going to be orphaned
		},
		{
			Number: big.NewInt(N + 2),
		},
	}

	for i := range l1Headers {
		err := s.l1Storage.StoreEvent(s.ctx, &Event{
			Hash:        getMsgHash(msgSourceSubscription, i),
			BlockNumber: l1Headers[i].Number.Uint64(),
			BlockHash:   l1Headers[i].Hash(),
		})
		s.Require().NoError(err)
		s.setBlockHeaderOnL1(l1Headers[i])
	}

	// overriding hash for N+1 block
	s.setBlockHeaderOnL1(ethtypes.Header{
		Number: big.NewInt(N + 1),
	})

	s.advanceFinalizedBlockNumberTo(N)
	s.eventListenerStub.waitForEnsurerLoop()
	s.checkL2StorageContent(N-1, N)

	s.advanceFinalizedBlockNumberTo(N + 100)
	s.eventListenerStub.waitForEnsurerLoop()
	s.checkL2StorageContent(N-1, N, N+2) // N+1 is not expected here

	err := s.l1Storage.IterateEventsByBatch(s.ctx, 100, func(events []*Event) error {
		s.Fail("some events unexpectedly left in L1 storage", "found %d events", len(events))
		return nil
	})
	s.Require().NoError(err)
}
