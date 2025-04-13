package l2

import (
	"context"
	"fmt"
	"testing"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/services/relayer/internal/storage"
	"github.com/jonboulle/clockwork"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/suite"
)

type eventFinalizerStub struct {
	emitter chan struct{}
}

func newEventFinalizerStub() *eventFinalizerStub {
	return &eventFinalizerStub{
		emitter: make(chan struct{}),
	}
}

// Can be used by reading routine to look for updates without further delay
func (efs *eventFinalizerStub) EventFinalized() <-chan struct{} {
	return efs.emitter
}

func (efs *eventFinalizerStub) emit() {
	efs.emitter <- struct{}{}
}

func (efs *eventFinalizerStub) waitForSenderLoop() {
	efs.emit()

	// channel is not buffered so by the moment
	// the emitter is able to exit from the second emit call
	// testing transaction sender must run at least one full loop iteration
	efs.emit()
}

type TransactionSenderTestSuite struct {
	suite.Suite

	// high-level dependencies
	database       db.DB
	logger         logging.Logger
	l2Storage      *EventStorage
	storageMetrics storage.TableMetrics

	// testing entity
	transactionSender        *TransactionSender
	transactionSenderMetrics TransactionSenderMetrics

	// mocks
	contractMock   *L2ContractMock
	clockMock      *clockwork.FakeClock
	eventFinalizer *eventFinalizerStub

	// test lifecycle stuff
	ctx                      context.Context
	cancel                   context.CancelFunc
	transactionSenderStopped chan struct{}
}

func TestTransactionSender(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(TransactionSenderTestSuite))
}

func (s *TransactionSenderTestSuite) SetupTest() {
	var err error

	s.ctx, s.cancel = context.WithCancel(context.Background())
	s.logger = logging.NewFromZerolog(zerolog.New(zerolog.NewConsoleWriter()))

	s.database, err = db.NewBadgerDbInMemory()
	s.Require().NoError(err, "failed to initialize database")

	s.clockMock = clockwork.NewFakeClock()

	s.contractMock = &L2ContractMock{}

	s.storageMetrics, err = storage.NewTableMetrics()
	s.Require().NoError(err)

	s.l2Storage = NewEventStorage(s.ctx, s.database, s.clockMock, s.storageMetrics, s.logger)

	cfg := DefaultTransactionSenderConfig()

	s.eventFinalizer = newEventFinalizerStub()

	s.transactionSenderMetrics, err = NewTransactionSenderMetrics()
	s.Require().NoError(err)

	s.transactionSender, err = NewTransactionSender(
		cfg,
		s.l2Storage,
		s.logger,
		s.clockMock,
		s.eventFinalizer,
		s.transactionSenderMetrics,
		s.contractMock,
	)
	s.Require().NoError(err, "failed to initialize transaction sender")
}

func (s *TransactionSenderTestSuite) TearDownTest() {
	s.cancel()
	<-s.transactionSenderStopped
}

func (s *TransactionSenderTestSuite) runSender() (context.CancelFunc, <-chan struct{}) {
	s.transactionSenderStopped = make(chan struct{})
	transactionSenderStarted := make(chan struct{})

	ctx, cancel := context.WithCancel(s.ctx)
	go func() {
		if err := s.transactionSender.Run(ctx, transactionSenderStarted); err != nil {
			s.ErrorIs(err, context.Canceled)
		}
		close(s.transactionSenderStopped)
	}()
	<-transactionSenderStarted

	return cancel, s.transactionSenderStopped
}

func (s *TransactionSenderTestSuite) runSenderWithExpectedEvents(sequenceNumbers []uint64, failOnSeqNo *uint64) {
	s.T().Helper()

	set := make(map[uint64]bool)
	for _, seqNo := range sequenceNumbers {
		s.Require().False(set[seqNo])
		set[seqNo] = false
	}

	cancel, stopped := s.runSender()

	var seqNoIdx int
	s.contractMock.RelayMessageFunc = func(ctx context.Context, event *Event) (common.Hash, error) {
		if failOnSeqNo != nil && event.SequenceNumber == *failOnSeqNo {
			return common.EmptyHash, fmt.Errorf("managed failure on %d seqno", event.SequenceNumber)
		}
		s.Require().Equal(
			sequenceNumbers[seqNoIdx], event.SequenceNumber,
			"unexpected order of events (seq number %d)", seqNoIdx,
		)
		seqNoIdx++
		set[event.SequenceNumber] = true
		return common.EmptyHash, nil
	}

	s.eventFinalizer.waitForSenderLoop()

	cancel()
	<-stopped

	for seqNo, invoked := range set {
		s.True(invoked, "found not forwarded event, seq no %d", seqNo)
	}
}

func (s *TransactionSenderTestSuite) TestBasic() {
	l2Events := []*Event{
		{
			Hash:           getMsgHash(1),
			SequenceNumber: 1,
		},
		{
			Hash:           getMsgHash(2),
			SequenceNumber: 2,
		},
		{
			Hash:           getMsgHash(3),
			SequenceNumber: 3,
		},
	}

	s.Require().NoError(s.l2Storage.StoreEvents(s.ctx, l2Events))

	s.runSenderWithExpectedEvents([]uint64{1, 2, 3}, nil)

	err := s.l2Storage.IterateEventsByBatch(s.ctx, 3, func(events []*Event) error {
		s.Fail("not expected events found in L2 event storage", "found %d events", len(events))
		return nil
	})
	s.Require().NoError(err)
}

func (s *TransactionSenderTestSuite) TestFailure() {
	l2Events := []*Event{
		{
			Hash:           getMsgHash(1),
			SequenceNumber: 1,
		},
		{
			Hash:           getMsgHash(2),
			SequenceNumber: 2,
		},
		{
			Hash:           getMsgHash(3),
			SequenceNumber: 3,
		},
		{
			Hash:           getMsgHash(4),
			SequenceNumber: 4,
		},
		{
			Hash:           getMsgHash(6),
			SequenceNumber: 6,
		},
	}

	s.Require().NoError(s.l2Storage.StoreEvents(s.ctx, l2Events))

	var failOnSeqNo uint64 = 4
	s.runSenderWithExpectedEvents([]uint64{1, 2, 3}, &failOnSeqNo)

	err := s.l2Storage.IterateEventsByBatch(s.ctx, 3, func(events []*Event) error {
		s.Require().Len(events, 2)
		s.Require().EqualValues(4, events[0].SequenceNumber)
		s.Require().EqualValues(6, events[1].SequenceNumber)
		return nil
	})
	s.Require().NoError(err)

	s.runSenderWithExpectedEvents([]uint64{4, 6}, nil)

	err = s.l2Storage.IterateEventsByBatch(s.ctx, 3, func(events []*Event) error {
		s.Fail("not expected events found in L2 event storage", "found %d events", len(events))
		return nil
	})
	s.Require().NoError(err)
}

func getMsgHash(seqNo int) [32]byte {
	var hash [32]byte
	for i := range hash {
		hash[i] = byte(seqNo)
	}
	return hash
}
