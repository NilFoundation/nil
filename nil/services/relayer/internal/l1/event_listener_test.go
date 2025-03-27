package l1

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/services/relayer/internal/storage"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
	"github.com/jonboulle/clockwork"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/suite"
)

type EventListenerTestSuite struct {
	suite.Suite

	// high level dependencies
	database       db.DB
	storage        *EventStorage
	storageMetrics storage.TableMetrics
	logger         logging.Logger
	clock          clockwork.Clock

	// testing entity
	listener        *EventListener
	listenerMetrics EventListenerMetrics

	// mocks
	ethClientMock  *EthClientMock
	l1ContractMock *L1ContractMock

	// testing lifecycle stuff
	ctx      context.Context
	canceler context.CancelFunc

	listenerCtx      context.Context
	listenerCanceler context.CancelFunc

	listenerStopped chan struct{}
}

func TestEventListener(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(EventListenerTestSuite))
}

func (s *EventListenerTestSuite) SetupTest() {
	var err error

	s.ctx, s.canceler = context.WithCancel(context.Background())
	s.logger = logging.NewFromZerolog(zerolog.New(zerolog.NewConsoleWriter()))

	s.database, err = db.NewBadgerDbInMemory()
	s.Require().NoError(err, "failed to initialize test db")

	s.listenerMetrics, err = NewEventListenerMetrics()
	s.Require().NoError(err)

	s.storageMetrics, err = storage.NewTableMetrics()
	s.Require().NoError(err)

	s.clock = clockwork.NewRealClock()
	s.ethClientMock = &EthClientMock{}
	s.l1ContractMock = &L1ContractMock{}
	s.storage, err = NewEventStorage(s.ctx, s.database, s.clock, s.storageMetrics, s.logger)
	s.Require().NoError(err, "failed to initialize event storage")

	cfg := DefaultEventListenerConfig()
	cfg.PollInterval = time.Millisecond
	cfg.BridgeMessengerContractAddress = "0xDEADBEEF"
	cfg.EmitEventCapacity = 100 // do avoid event dropping

	s.listener, err = NewEventListener(
		cfg,
		s.clock,
		s.ethClientMock,
		s.l1ContractMock,
		s.storage,
		s.listenerMetrics,
		s.logger,
	)
	s.Require().NoError(err, "failed to create listener")
}

func (s *EventListenerTestSuite) runListener() {
	s.listenerCtx, s.listenerCanceler = context.WithCancel(s.ctx)

	listenerStarted := make(chan struct{})
	s.listenerStopped = make(chan struct{})
	go func() {
		defer close(s.listenerStopped)
		err := s.listener.Run(s.listenerCtx, listenerStarted)
		if err != nil {
			s.ErrorIs(err, context.Canceled)
		}
	}()

	<-listenerStarted
}

func (s *EventListenerTestSuite) stopListener() {
	if s.listenerCanceler != nil {
		s.listenerCanceler()
	}
	<-s.listenerStopped
}

func (s *EventListenerTestSuite) waitForEvents(eventCount int) chan struct{} {
	done := make(chan struct{})
	go func() {
		for range eventCount {
			<-s.listener.EventReceived()
		}
		close(done)
	}()
	return done
}

func (s *EventListenerTestSuite) TearDownTest() {
	s.canceler()
	<-s.listenerStopped
}

func (s *EventListenerTestSuite) TestEmptyRun() {
	// some default block value
	s.ethClientMock.HeaderByNumberFunc = func(ctx context.Context, number *big.Int) (*ethtypes.Header, error) {
		return &ethtypes.Header{Number: big.NewInt(1024)}, nil
	}

	// default subscription initializer
	s.l1ContractMock.SubscribeToEventsFunc = func(
		ctx context.Context,
		sink chan<- *L1MessageSent,
	) (event.Subscription, error) {
		return event.NewSubscription(func(<-chan struct{}) error {
			return nil
		}), nil
	}

	s.runListener()
}

func (s *EventListenerTestSuite) TestFetchHistoricalEvents() {
	// test case:
	// set latest block to 1024
	// set last processed block to 800
	// return events for blocks 801, 901, 1001 (using fetcher's request mock)
	// ensure their content and order in storage
	// repeat the test from the beginning (with modified database)

	s.ethClientMock.HeaderByNumberFunc = func(ctx context.Context, number *big.Int) (*ethtypes.Header, error) {
		return &ethtypes.Header{Number: big.NewInt(1024)}, nil
	}
	s.l1ContractMock.SubscribeToEventsFunc = func(
		ctx context.Context,
		sink chan<- *L1MessageSent,
	) (event.Subscription, error) {
		return event.NewSubscription(func(<-chan struct{}) error {
			return nil
		}), nil
	}

	expectedRanges := []struct {
		from, to uint64
	}{
		{801, 900},
		{901, 1000},
		{1001, 1024},
	}

	testIteration := func() {
		defer s.stopListener()

		callNumber := 0
		s.l1ContractMock.GetEventsFromBlockRangeFunc = func(
			ctx context.Context,
			from uint64,
			to *uint64,
		) ([]*L1MessageSent, error) {
			s.Equal(from, expectedRanges[callNumber].from, "bad call number %d", callNumber)
			if s.NotNil(to) {
				s.Equal(*to, expectedRanges[callNumber].to, "bad call number %d", callNumber)
			}
			callNumber++

			// for each range return single event for its first block
			return []*L1MessageSent{
				{
					MessageHash: getMsgHash(msgSourceFetcher, callNumber+1),
					Raw: types.Log{
						BlockNumber: from,
						BlockHash:   ethcommon.BytesToHash([]byte{1, 2, 3, 4}),
					},
				},
			}, nil
		}

		err := s.storage.SetLastProcessedBlock(s.ctx, &ProcessedBlock{
			BlockNumber: 800, // [800; 1024) blocks are expected to be fetched
			BlockHash:   ethcommon.BytesToHash([]byte{1, 2, 3, 4}),
		})
		s.Require().NoError(err)

		eventCount := len(expectedRanges)

		awaiter := s.waitForEvents(eventCount)
		s.runListener()
		<-awaiter

		err = s.storage.IterateEventsByBatch(s.ctx, 100, func(events []*Event) error {
			s.Len(events, eventCount)
			for i, event := range events {
				s.EqualValues(expectedRanges[i].from, event.BlockNumber)
				s.EqualValues(i, event.SequenceNumber)
			}
			return nil
		})
		s.Require().NoError(err, "failed to iterate saved events")

		processedBlock, err := s.storage.GetLastProcessedBlock(s.ctx)
		s.Require().NoError(err)

		// we still might receive some updates from last block so last processed now is the one before las
		s.EqualValues(901, processedBlock.BlockNumber)
	}

	testIteration()

	// Run sequence again setting last block to the same 800
	// Target is to check that ordering is not changed (and nothing is stuck on repeat)
	s.Run("Idempotent", func() {
		testIteration()
	})
}

func (s *EventListenerTestSuite) TestFetchEventsFromSubscription() {
	// test case:
	// set latest block to 1024
	// push events for subscription to blocks 1025, 1026, 1027
	// ensure their content and order in storage
	// repeat the test from the beginning (with modified database)

	s.ethClientMock.HeaderByNumberFunc = func(ctx context.Context, number *big.Int) (*ethtypes.Header, error) {
		return &ethtypes.Header{Number: big.NewInt(1024)}, nil
	}

	testIteration := func() {
		defer s.stopListener()

		// mock subscription to provide new events
		s.l1ContractMock.SubscribeToEventsFunc = func(
			ctx context.Context,
			sink chan<- *L1MessageSent,
		) (event.Subscription, error) {
			sub := event.NewSubscription(func(<-chan struct{}) error {
				<-ctx.Done()
				return nil
			})

			go func() {
				for i := 1; i < 4; i++ {
					sink <- &L1MessageSent{
						MessageHash: getMsgHash(msgSourceSubscription, i),
						Raw: types.Log{
							BlockNumber: 1024 + uint64(i),
							BlockHash:   ethcommon.BytesToHash([]byte{1, 2, 3, byte(i)}),
						},
					}
				}
			}()

			return sub, nil
		}

		eventCount := 3

		awaiter := s.waitForEvents(eventCount)
		s.runListener()
		<-awaiter

		err := s.storage.IterateEventsByBatch(s.ctx, 100, func(events []*Event) error {
			s.Len(events, eventCount)
			for i, event := range events {
				s.EqualValues(1024+i+1, event.BlockNumber)
				s.EqualValues(i, event.SequenceNumber)
			}
			return nil
		})
		s.Require().NoError(err, "failed to iterate saved events")

		lastProcessedBlock, err := s.storage.GetLastProcessedBlock(s.ctx)
		s.Require().NoError(err)

		s.EqualValues(1026, lastProcessedBlock.BlockNumber)
	}

	testIteration()

	s.Run("Idempotent", func() {
		testIteration()
	})
}

func (s *EventListenerTestSuite) TestSmoke() {
	// test case:
	// set current block number to 1024
	// set last processed block number to 800
	// run whole listener, simultaneously push events to fetcher and subscriber
	// ensure that all events are stored in the given order (first from fetcher, then from subscriber)

	s.ethClientMock.HeaderByNumberFunc = func(ctx context.Context, number *big.Int) (*ethtypes.Header, error) {
		return &ethtypes.Header{Number: big.NewInt(1024)}, nil
	}

	s.l1ContractMock.SubscribeToEventsFunc = func(
		ctx context.Context,
		sink chan<- *L1MessageSent,
	) (event.Subscription, error) {
		sub := event.NewSubscription(func(<-chan struct{}) error {
			<-ctx.Done()
			return nil
		})

		go func() {
			for i := 1; i < 4; i++ {
				sink <- &L1MessageSent{
					MessageHash: getMsgHash(msgSourceSubscription, i),
					Raw: types.Log{
						BlockNumber: 1024 + uint64(i),
						BlockHash:   ethcommon.BytesToHash([]byte{1, 2, 3, byte(i)}),
					},
				}
			}
		}()

		return sub, nil
	}

	expectedRanges := []struct {
		from, to uint64
	}{
		{801, 900},
		{901, 1000},
		{1001, 1024},
	}

	callNumber := 0
	s.l1ContractMock.GetEventsFromBlockRangeFunc = func(
		ctx context.Context,
		from uint64,
		to *uint64,
	) ([]*L1MessageSent, error) {
		s.Equal(from, expectedRanges[callNumber].from, "bad call number %d", callNumber)
		if s.NotNil(to) {
			s.Equal(*to, expectedRanges[callNumber].to, "bad call number %d", callNumber)
		}
		callNumber++

		// for each range return single event for its first block
		return []*L1MessageSent{
			{
				MessageHash: getMsgHash(msgSourceFetcher, callNumber+1),
				Raw: types.Log{
					BlockNumber: from,
					BlockHash:   ethcommon.BytesToHash([]byte{1, 2, 3, 4}),
				},
			},
		}, nil
	}

	err := s.storage.SetLastProcessedBlock(s.ctx, &ProcessedBlock{
		BlockNumber: 800, // [800; 1024) blocks are expected to be fetched
		BlockHash:   ethcommon.BytesToHash([]byte{1, 2, 3, 4}),
	})
	s.Require().NoError(err)

	eventCount := 6
	awaiter := s.waitForEvents(eventCount)
	s.runListener()
	<-awaiter

	err = s.storage.IterateEventsByBatch(s.ctx, 100, func(events []*Event) error {
		s.Require().Len(events, 6)

		expectedBlockNumbers := [6]int{801, 901, 1001, 1025, 1026, 1027}
		for i, n := range expectedBlockNumbers {
			s.EqualValues(i, events[i].SequenceNumber)
			s.EqualValues(n, events[i].BlockNumber)
		}

		return nil
	})
	s.Require().NoError(err)
}

type msgSource byte

const (
	msgSourceFetcher      msgSource = 0
	msgSourceSubscription msgSource = 1
)

func getMsgHash(source msgSource, seqNo int) [32]byte {
	var hash [32]byte
	hash[0] = byte(source)
	for i := range hash[1:] {
		hash[i+1] = byte(seqNo)
	}
	return hash
}

// TODO(oclaw) add checks for shutdown
// TODO(oclaw) add checks for event data filling
