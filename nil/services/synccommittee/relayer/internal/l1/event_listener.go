package l1

import (
	"context"
	"errors"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/rs/zerolog"
	"golang.org/x/sync/errgroup"
)

type EventListenerConfig struct {
	BridgeMessengerContractAddress common.Address

	// settings for historical events fetcher
	BatchSize    int
	PollInterval time.Duration
}

func DefaultEventListenerConfig() *EventListenerConfig {
	return &EventListenerConfig{
		BatchSize:    100,
		PollInterval: time.Millisecond * 100,
	}
}

func (cfg *EventListenerConfig) Validate() error {
	var emptyAddr common.Address
	if cfg.BridgeMessengerContractAddress == emptyAddr {
		return errors.New("empty L1BridgeMessenger contract addr")
	}
	if cfg.BatchSize == 0 {
		return errors.New("empty batch size for fetching old events")
	}
	if cfg.PollInterval == 0 {
		return errors.New("empty poll interval for fetching old events")
	}
	return nil
}

type EventListener struct {
	ethClient    ethclient.Client
	config       *EventListenerConfig
	eventStorage *EventStorage

	state struct {
		emitter            chan struct{} // signals when new event is put to storage
		currentBlockNumber uint64        // last block event from which is put to storage
	}

	logger zerolog.Logger
}

func NewEventListener(
	ethClient ethclient.Client,
	config *EventListenerConfig,
	storage *EventStorage,
	logger zerolog.Logger,
) *EventListener {
	el := &EventListener{
		ethClient:    ethClient,
		config:       config,
		eventStorage: storage,
		logger:       logger,
	}

	el.state.emitter = make(chan struct{})

	return el
}

func (el *EventListener) Name() string {
	return "relayer-l1-event-listener"
}

func (el *EventListener) Run(ctx context.Context, started chan<- struct{}) error {
	if err := el.config.Validate(); err != nil {
		return err
	}

	var (
		oldEventCh = make(chan types.Log, el.config.BatchSize)
		newEventCh = make(chan types.Log, el.config.BatchSize*10) // large buffer to keep accumulating new events while fetching last (ehtereum.Client anyway uses large internal buffers
	)

	// 1. Subscribe to new events (done before fetching old ones in order to avoid event loss)
	subscription, err := el.subscribeToNewEvents(ctx, newEventCh)
	if err != nil {
		return err
	}
	defer subscription.Unsubscribe()

	eg, gCtx := errgroup.WithContext(ctx)

	// 2. Start fetching historical events by batches (as soon as we reach block from which listener was started - routine ends)
	eg.Go(func() error {
		return el.fetchPastEvents(gCtx, oldEventCh)
	})

	// 3. Process incoming events ordered (historical events go first)
	eg.Go(func() error {
		return el.recvEvents(gCtx, oldEventCh, newEventCh)
	})

	close(started)

	return eg.Wait()
}

func (el *EventListener) EventReceived() <-chan struct{} {
	return el.state.emitter
}

func (el *EventListener) subscribeToNewEvents(ctx context.Context, logCh chan types.Log) (ethereum.Subscription, error) {
	query := ethereum.FilterQuery{
		Addresses: []common.Address{
			el.config.BridgeMessengerContractAddress,
		},
		// TODO(oclaw): add topic for MessageSentEvent
	}

	sub, err := el.ethClient.SubscribeFilterLogs(ctx, query, logCh)
	if err != nil {
		return nil, err
	}

	el.logger.Info().
		Str("l1_bridge_messenger_addr", el.config.BridgeMessengerContractAddress.Hex()).
		Msg("subscribed to new events")

	return sub, nil
}

func (el *EventListener) fetchPastEvents(
	ctx context.Context,
	logCh chan types.Log,
) error {
	defer close(logCh)

	header, err := el.ethClient.HeaderByNumber(ctx, nil)
	if err != nil {
		return err
	}
	latestBlock := header.Number.Uint64()

	lastProcessedBlock, err := el.eventStorage.GetLastProcessedBlock(ctx)
	if err != nil {
		return err
	}

	lastProcessedBlockNum := latestBlock // if no last processed block num is in storage - start from the current one
	if lastProcessedBlock != nil {
		lastProcessedBlockNum = lastProcessedBlock.BlockNumber.Uint64()
	}

	el.logger.Info().
		Uint64("latest_block_num", latestBlock).
		Uint64("latest_processed_block_num", lastProcessedBlockNum).
		Msg("connected to Etherium")

	if lastProcessedBlockNum >= latestBlock {
		el.logger.Info().Msg("no need to fetch old events")
	}

	query := ethereum.FilterQuery{
		Addresses: []common.Address{
			el.config.BridgeMessengerContractAddress,
		},
		// TODO(oclaw) add topic for MessageSent event
	}

	ticker := time.NewTicker(el.config.PollInterval) // TODO(oclaw) add mock for ticker
	batchSize := uint64(el.config.BatchSize)
	for fromBlock := lastProcessedBlockNum + 1; fromBlock <= latestBlock; fromBlock += batchSize {
		select {
		case <-ticker.C:
		case <-ctx.Done():
			return ctx.Err()
		}

		toBlock := min(latestBlock, fromBlock+batchSize-1)
		query.FromBlock = big.NewInt(int64(fromBlock))
		query.ToBlock = big.NewInt(int64(toBlock))

		el.logger.Info().
			Uint64("block_range_start", fromBlock).
			Uint64("block_range_end", toBlock).
			Msg("fetching historical events from block rang")

		logs, err := el.ethClient.FilterLogs(ctx, query)
		if err != nil {
			return err
		}

		for _, log := range logs {
			logCh <- log
		}
	}
	return nil
}

func (el *EventListener) recvEvents(
	ctx context.Context,
	oldEventChan chan types.Log,
	newEventChan chan types.Log,
) error {
	el.logger.Info().Msg("started processing incoming events")
	defer func() {
		el.logger.Info().Msg("finished processing incoming events")
	}()

	processedOldEvents := 0
	for event := range oldEventChan {
		if err := el.processEvent(ctx, event); err != nil {
			return err
		}
		processedOldEvents++
	}

	el.logger.Info().
		Int("old_processed_events", processedOldEvents).
		Int("incoming_events_buf", len(newEventChan)).
		Msg("all old events fetched")

	for {
		select {
		case event, ok := <-newEventChan:
			if !ok {
				return nil // subscription is inactive now
			}

			if err := el.processEvent(ctx, event); err != nil {
				return err
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (el *EventListener) processEvent(ctx context.Context, ethEvent types.Log) error {
	event, err := el.decodeEventPayload(ethEvent)
	if err != nil {
		return err
	}

	// TODO(oclaw) do we need to retry at this point?
	// TODO(oclaw) ignore duplicates?
	if err := el.eventStorage.StoreEvent(ctx, event); err != nil {
		return err
	}

	if el.state.currentBlockNumber != ethEvent.BlockNumber {
		el.logger.Info().Uint64("block_number", el.state.currentBlockNumber).Msg("finished processing events from block")
		if err := el.storeLastProcessedBlock(ctx); err != nil {
			return err
		}
		el.state.currentBlockNumber = ethEvent.BlockNumber
	}

	// non-blocking write to notify reader
	select {
	case el.state.emitter <- struct{}{}:
	default:
	}

	return nil
}

func (el *EventListener) decodeEventPayload(ethEvent types.Log) (*Event, error) {
	// TODO(oclaw) read event data according to the L1BridgeMessenger contract ABI
	return nil, errors.New("not implemented")
}

func (el *EventListener) storeLastProcessedBlock(ctx context.Context) error {
	if el.state.currentBlockNumber == 0 {
		return nil
	}

	var procBlk ProcessedBlock
	// TODO(oclaw) fill processed block from el.state
	return el.eventStorage.SetLastProcessedBlock(ctx, &procBlk)
}
