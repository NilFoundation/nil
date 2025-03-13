package l1

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/db"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/suite"
)

type EventListenerTestSuite struct {
	suite.Suite

	// high level dependencies
	database db.DB
	storage  *EventStorage
	logger   zerolog.Logger

	// testing entity
	listener *EventListener

	// mocks
	ethClientMock  *EthClientMock
	l1ContractMock *L1ContractMock
	clockMock      *common.TestTimerImpl

	// testing lifecycle stuff
	ctx             context.Context
	canceler        context.CancelFunc
	listenerStopped chan struct{}
}

func TestEventListener(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(EventListenerTestSuite))
}

func (s *EventListenerTestSuite) SetupTest() {
	var err error

	s.ctx, s.canceler = context.WithCancel(context.Background())
	s.logger = zerolog.New(zerolog.NewConsoleWriter())

	s.database, err = db.NewBadgerDbInMemory()
	s.Require().NoError(err, "failed to initialize test db")

	s.clockMock = common.NewTestTimerFromTime(time.Now())
	s.ethClientMock = &EthClientMock{}
	s.l1ContractMock = &L1ContractMock{}

	s.storage, err = NewEventStorage(s.ctx, s.database, s.clockMock, nil, s.logger)
	s.Require().NoError(err, "failed to initialize event storage")

	cfg := DefaultEventListenerConfig()
	cfg.BridgeMessengerContractAddress = "0xDEADBEEF"

	s.listener, err = NewEventListener(cfg, s.ethClientMock, s.l1ContractMock, s.storage, s.logger)
	s.Require().NoError(err, "failed to create listener")
}

func (s *EventListenerTestSuite) runListener() {
	listenerStarted := make(chan struct{})
	s.listenerStopped = make(chan struct{})
	go func() {
		defer close(s.listenerStopped)
		err := s.listener.Run(s.ctx, listenerStarted)
		if err != nil {
			s.ErrorIs(err, context.Canceled)
		}
	}()

	<-listenerStarted
}

func (s *EventListenerTestSuite) waitForEvents(eventCount int) chan struct{} {
	done := make(chan struct{})
	go func() {
		for range eventCount {
			<-s.listener.EventReceived()
		}
		done <- struct{}{}
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
	s.l1ContractMock.SubscribeToEventsFunc = func(ctx context.Context, sink chan<- *L1MessageSent) (event.Subscription, error) {
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
	// return events for blocks 801, 901, 1001
	// ensure their content and order in storage

	s.ethClientMock.HeaderByNumberFunc = func(ctx context.Context, number *big.Int) (*ethtypes.Header, error) {
		return &ethtypes.Header{Number: big.NewInt(1024)}, nil
	}
	s.l1ContractMock.SubscribeToEventsFunc = func(ctx context.Context, sink chan<- *L1MessageSent) (event.Subscription, error) {
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

	callNumber := 0
	s.l1ContractMock.GetEventsFromBlockRangeFunc = func(ctx context.Context, from uint64, to *uint64) ([]*L1MessageSent, error) {
		s.Equal(from, expectedRanges[callNumber].from, "bad call number %d", callNumber)
		if s.NotNil(to) {
			s.Equal(*to, expectedRanges[callNumber].to, "bad call number %d", callNumber)
		}
		callNumber++

		var msgHash [32]byte
		for i := range msgHash {
			msgHash[i] = byte(from)
		}

		// for each range return single event for its first block
		return []*L1MessageSent{
			{
				MessageHash: msgHash,
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
		for _, event := range events {
			switch event.BlockNumber {
			case 801:
				s.EqualValues(0, event.SequenceNumber)
			case 901:
				s.EqualValues(1, event.SequenceNumber)
			case 1001:
				s.EqualValues(2, event.SequenceNumber)
			default:
				s.Fail("unexpected block number in event", "block number %d", event.BlockNumber)
			}
		}
		return nil
	})
	s.Require().NoError(err, "failed to iterate saved events")
}

func (s *EventListenerTestSuite) TestFetchEventsFromSubscription() {
	// TODO (oclaw) subscription event fetching test
	// TODO (oclaw) subscription event fetching error & retry after error
	s.True(false, "implement me!")
}

func (s *EventListenerTestSuite) TestSmoke() {
	// TODO (oclaw) parallel fetching test with mandatory order check
	s.True(false, "implement me!")
}

// TODO(oclaw) add checks for shutdown
