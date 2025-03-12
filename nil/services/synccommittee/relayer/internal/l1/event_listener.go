package l1

import (
	"context"
	"errors"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/rs/zerolog"
	"golang.org/x/sync/errgroup"
)

type EventListenerConfig struct {
	BridgeMessengerContractAddress string

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
	if cfg.BridgeMessengerContractAddress == "" {
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
	rawEthClient     *ethclient.Client
	contractBindning *L1

	config       *EventListenerConfig
	eventStorage *EventStorage

	state struct {
		emitter chan struct{} // signals when new event is put to storage

		// last block event from which is put to storage
		currentBlockNumber uint64
		currentBlockHash   common.Hash
	}

	logger zerolog.Logger
}

func NewEventListener(
	ethClient *ethclient.Client, // TODO replace with interface
	config *EventListenerConfig,
	storage *EventStorage,
	logger zerolog.Logger,
) (*EventListener, error) {
	addr := common.HexToAddress(config.BridgeMessengerContractAddress)
	binding, err := NewL1(addr, ethClient)
	if err != nil {
		return nil, err
	}

	el := &EventListener{
		rawEthClient:     ethClient,
		contractBindning: binding,
		config:           config,
		eventStorage:     storage,
		logger:           logger,
	}

	el.state.emitter = make(chan struct{})

	return el, nil
}

func (el *EventListener) Name() string {
	return "relayer-l1-event-listener"
}

func (el *EventListener) Run(ctx context.Context, started chan<- struct{}) error {
	if err := el.config.Validate(); err != nil {
		return err
	}

	var (
		oldEventCh = make(chan *L1MessageSent, el.config.BatchSize)
		newEventCh = make(chan *L1MessageSent, el.config.BatchSize*10) // large buffer to keep accumulating new events while fetching last (ehtereum.Client anyway uses large internal buffers
	)

	eg, gCtx := errgroup.WithContext(ctx)

	// 1. Subscribe to new events (done before fetching old ones in order to avoid event loss)
	// gCtx is passed in order to kill subscription properly in case of error in any sub routine
	subscription, err := el.subscribeToNewEvents(gCtx, newEventCh)
	if err != nil {
		return err
	}
	defer subscription.Unsubscribe()

	// 2. Start fetching historical events by batches (as soon as we reach block from which listener was started - routine ends)
	eg.Go(func() error {
		return el.fetchPastEvents(ctx, oldEventCh)
	})

	// 3. Process incoming events ordered (historical events go first)
	eg.Go(func() error {
		return el.recvEvents(gCtx, oldEventCh, newEventCh, subscription)
	})

	close(started) // started == successfully subscribed to notifications from L1 contract

	return eg.Wait()
}

// Can be used by reading routine to look for updates without further delay
func (el *EventListener) EventReceived() <-chan struct{} {
	return el.state.emitter
}

func (el *EventListener) subscribeToNewEvents(ctx context.Context, eventCh chan<- *L1MessageSent) (ethereum.Subscription, error) {
	sub, err := el.contractBindning.WatchMessageSent(
		&bind.WatchOpts{Context: ctx},
		eventCh,
		// TODO(oclaw) do we need filters?
		nil, // messageSender []common.Address,
		nil, // messageTarget []common.Address,
		nil, // messageNonce []*big.Int
	)
	if err != nil {
		return nil, err
	}

	el.logger.Info().
		Str("l1_bridge_messenger_addr", el.config.BridgeMessengerContractAddress).
		Msg("subscribed to new events")

	return sub, nil
}

func (el *EventListener) fetchPastEvents(ctx context.Context, eventCh chan<- *L1MessageSent) error {
	defer close(eventCh) // no more events will be posted to the channel after routine finished its work
	for {
		err := el.doFetchPastEvents(ctx, eventCh)
		if err == nil {
			return nil
		}
		if errors.Is(err, context.DeadlineExceeded) {
			el.logger.Warn().Err(err).Msg("historical event fetching timed out")
			continue
		}
		return err
	}
}

func (el *EventListener) doFetchPastEvents(ctx context.Context, eventCh chan<- *L1MessageSent) error {
	header, err := el.rawEthClient.HeaderByNumber(ctx, nil)
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
		lastProcessedBlockNum = lastProcessedBlock.BlockNumber
	}

	el.logger.Info().
		Uint64("latest_block_num", latestBlock).
		Uint64("latest_processed_block_num", lastProcessedBlockNum).
		Msg("connected to Etherium")

	if lastProcessedBlockNum >= latestBlock {
		el.logger.Info().Msg("no need to fetch old events")
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

		el.logger.Info().
			Uint64("block_range_start", fromBlock).
			Uint64("block_range_end", toBlock).
			Msg("fetching historical events from block rang")

		iter, err := el.contractBindning.FilterMessageSent(
			&bind.FilterOpts{
				Start: fromBlock,
				End:   &toBlock,
			},
			// TODO(oclaw) do we need filters?
			nil, // messageSender []common.Address,
			nil, // messageTarget []common.Address,
			nil, // messageNonce []*big.Int
		)
		if err != nil {
			return err
		}

		for iter.Next() {
			eventCh <- iter.Event
		}
		if err := iter.Error(); err != nil {
			return err
		}
	}
	return nil
}

func (el *EventListener) recvEvents(
	ctx context.Context,
	oldEventChan chan *L1MessageSent,
	newEventChan chan *L1MessageSent,
	subscription ethereum.Subscription,
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
		Msg("finished processing old events")

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
		case err := <-subscription.Err():
			return err // raised when subscription is considered dead inside eth client
		}
	}
}

func (el *EventListener) processEvent(ctx context.Context, ethEvent *L1MessageSent) error {
	event, err := el.convertEvent(ethEvent)
	if err != nil {
		return err
	}

	if err := el.eventStorage.StoreEvent(ctx, event); err != nil {
		return err
	}

	if el.state.currentBlockNumber != ethEvent.Raw.BlockNumber {
		el.logger.Info().Uint64("block_number", el.state.currentBlockNumber).Msg("finished processing events from block")
		if err := el.onNewBlockBegan(ctx, ethEvent); err != nil {
			return err
		}
	}

	// non-blocking write to notify reader
	select {
	case el.state.emitter <- struct{}{}:
	default:
	}

	return nil
}

func (el *EventListener) convertEvent(ethEvent *L1MessageSent) (*Event, error) {
	event := &Event{
		Hash:        ethEvent.MessageHash,
		BlockNumber: ethEvent.Raw.BlockNumber,
		BlockHash:   ethEvent.Raw.BlockHash,

		Sender:             ethEvent.MessageSender,
		Target:             ethEvent.MessageTarget,
		Value:              ethEvent.MessageValue,
		Message:            ethEvent.Message,
		DepositType:        ethEvent.DepositType,
		CreatedAt:          ethEvent.MessageCreatedAt,
		ExpiryTime:         ethEvent.MessageExpiryTime,
		L2FeeRefundAddress: ethEvent.L2FeeRefundAddress,
		FeeCreditData: FeeCreditData{
			NilGasLimit:          ethEvent.FeeCreditData.NilGasLimit,
			MaxFeePerGas:         ethEvent.FeeCreditData.MaxFeePerGas,
			MaxPriorityFeePerGas: ethEvent.FeeCreditData.MaxPriorityFeePerGas,
			FeeCredit:            ethEvent.FeeCreditData.FeeCredit,
		},
	}
	return event, errors.New("event parsing not implemented")
}

func (el *EventListener) onNewBlockBegan(ctx context.Context, newBlockInfo *L1MessageSent) error {
	if el.state.currentBlockNumber == 0 {
		return nil
	}

	procBlk := &ProcessedBlock{
		BlockNumber: el.state.currentBlockNumber,
		BlockHash:   el.state.currentBlockHash,
	}

	if err := el.eventStorage.SetLastProcessedBlock(ctx, procBlk); err != nil {
		return err
	}

	el.state.currentBlockNumber = newBlockInfo.Raw.BlockNumber
	el.state.currentBlockHash = newBlockInfo.Raw.BlockHash
	return nil
}
