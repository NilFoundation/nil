package l1

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/db"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/suite"
)

type EventListenerTestSuite struct {
	suite.Suite

	// aux entities
	database db.DB
	storage  *EventStorage
	logger   zerolog.Logger

	// testing entity
	listener *EventListener

	// mocks
	ethClientMock  *EthClientMock
	l1ContractMock *L1ContractMock
	clockMock      *common.TestTimerImpl

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
		s.ErrorIs(err, context.Canceled)
	}()

	<-listenerStarted
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
	s.True(false, "implement me!")
	// TODO (oclaw) fetch historical events test
	// TODO (oclaw) fetch historical events error & retry after error
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
