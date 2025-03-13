package l1

import (
	"context"
	"errors"
	"time"

	"github.com/NilFoundation/nil/nil/common"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rpc"
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
	rawEthClient    EthClient
	contractBinding L1Contract

	config       *EventListenerConfig
	eventStorage *EventStorage

	state struct {
		emitter chan struct{} // signals when new event is put to storage

		// last block event from which is put to storage
		currentBlockNumber uint64
		currentBlockHash   ethcommon.Hash
	}

	logger zerolog.Logger
}

func NewEventListener(
	config *EventListenerConfig,
	ethClient EthClient,
	contractClient L1Contract,
	storage *EventStorage,
	logger zerolog.Logger,
) (*EventListener, error) {
	el := &EventListener{
		rawEthClient:    ethClient,
		contractBinding: contractClient,
		config:          config,
		eventStorage:    storage,
		logger:          logger,
	}

	el.state.emitter = make(chan struct{})

	return el, nil
}

func (el *EventListener) Name() string {
	return "l1-event-listener"
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

	eg.Go(func() error {
		return el.subscriber(gCtx, newEventCh)
	})

	eg.Go(func() error {
		return el.fetcher(gCtx, oldEventCh)
	})

	eg.Go(func() error {
		return el.eventProcessor(gCtx, oldEventCh, newEventCh)
	})

	close(started) // started == successfully subscribed to notifications from L1 contract

	err := eg.Wait()
	el.logger.Debug().Err(err).Msg("l1 event listener done")
	return err
}

// Can be used by reading routine to look for updates without further delay
func (el *EventListener) EventReceived() <-chan struct{} {
	return el.state.emitter
}

// Routine responsible for:
// - subscription to the contract events
// - tracking of the subscription status
// - reestablishing subscription in case of need
func (el *EventListener) subscriber(ctx context.Context, eventCh chan<- *L1MessageSent) error {
	defer close(eventCh)

	retrier := common.NewRetryRunner(
		common.RetryConfig{
			ShouldRetry: common.ComposeRetryPolicies(
				common.LimitRetries(100),
				common.DoNotRetryIf(
					rpc.ErrNotificationsUnsupported,  // not going to work with this connection
					rpc.ErrSubscriptionQueueOverflow, // receiver part is probably dead
				),
			),
			NextDelay: common.DelayExponential(time.Second, 15*time.Second),
		},
		el.logger,
	)

	// here goes an issue: 100 retries of subscription in a week for example will still exhaust
	// retry limit and lead to service shutdown, but should be OK in combination with external service restarts
	return retrier.Do(ctx, func(ctx context.Context) error {
		sub, err := el.contractBinding.SubscribeToEvents(ctx, eventCh)
		if err != nil {
			el.logger.Error().
				Str("contract_addr", el.config.BridgeMessengerContractAddress).
				Err(err).
				Msg("failed to subscribe to updates from L1 contract")
			return err

			// TODO(oclaw) metrics
		}
		defer sub.Unsubscribe()

		el.logger.Info().
			Str("contract_addr", el.config.BridgeMessengerContractAddress).
			Msg("subscribed to new events")

		select {
		case <-ctx.Done():
			el.logger.Debug().Msg("subscriber canceled")
			return ctx.Err()
		case err, ok := <-sub.Err(): // here we will try to reconnect
			if ok {
				el.logger.Error().
					Str("contract_addr", el.config.BridgeMessengerContractAddress).
					Err(err).
					Msg("L1 subscription is broken")
				return err

				// TODO(oclaw) metrics
			}
			el.logger.Debug().Msg("subscription channel is closed")
		}

		return nil
	})
}

// Routine responsible for fetching historical blocks in range [lastProcessedBlock; latest)
// until it is executed incoming event processing is shutdown
func (el *EventListener) fetcher(ctx context.Context, eventCh chan<- *L1MessageSent) error {
	defer close(eventCh) // no more events will be posted to the channel after routine finished its work

	// try fetching as long as possible, force exit after large enough attempt number
	retrier := common.NewRetryRunner(
		common.RetryConfig{
			ShouldRetry: common.LimitRetries(100),
			NextDelay:   common.DelayExponential(time.Second, time.Second*15),
		},
		el.logger,
	)

	return retrier.Do(ctx, func(ctx context.Context) error {
		err := el.fetchPastEvents(ctx, eventCh)
		if err != nil {
			el.logger.Error().Err(err).Msg("historical event fetching failed")
			// TODO (oclaw) metrics
		}
		return err
	})

	// TODO(oclaw) metrics (this routine is not expected to run for too long, we should now if something is stuck here)
}

func (el *EventListener) fetchPastEvents(ctx context.Context, eventCh chan<- *L1MessageSent) error {
	header, err := el.rawEthClient.HeaderByNumber(ctx, nil)
	if err != nil {
		return err
	}
	latestBlock := header.Number.Uint64()

	lastProcessedBlock, err := el.eventStorage.GetLastProcessedBlock(ctx)
	if err != nil {
		return err
	}

	if lastProcessedBlock != nil {
		el.setCurrentProcessingBlock(lastProcessedBlock.BlockNumber, lastProcessedBlock.BlockHash)
	} else {
		el.setCurrentProcessingBlock(latestBlock, header.Hash())
	}

	el.logger.Info().
		Uint64("latest_block_num", latestBlock).
		Uint64("latest_processed_block_num", el.state.currentBlockNumber).
		Msg("connected to Etherium")

	if el.state.currentBlockNumber >= latestBlock {
		el.logger.Info().Msg("no need to fetch old events")
	}

	ticker := time.NewTicker(el.config.PollInterval) // TODO(oclaw) add mock for ticker
	batchSize := uint64(el.config.BatchSize)
	for fromBlock := el.state.currentBlockNumber + 1; fromBlock <= latestBlock; fromBlock += batchSize {
		select {
		case <-ticker.C:
		case <-ctx.Done():
			return ctx.Err()
		}

		toBlock := min(latestBlock, fromBlock+batchSize-1)

		el.logger.Info().
			Uint64("block_range_start", fromBlock).
			Uint64("block_range_end", toBlock).
			Msg("fetching historical events from block range")

		events, err := el.contractBinding.GetEventsFromBlockRange(ctx, fromBlock, &toBlock)
		if err != nil {
			return err
		}

		for _, event := range events {
			eventCh <- event
		}
	}
	return nil
}

func (el *EventListener) eventProcessor(
	ctx context.Context,
	oldEventChan chan *L1MessageSent,
	newEventChan chan *L1MessageSent,
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
		}
	}
}

func (el *EventListener) processEvent(ctx context.Context, ethEvent *L1MessageSent) error {
	event := el.convertEvent(ethEvent)

	// all retryable errors should be handled inside storage, otherwise we should interrupt service work
	err := el.eventStorage.StoreEvent(ctx, event)
	if err := ignoreErrors(err, ErrKeyExists); err != nil {
		return err
	}

	if el.state.currentBlockNumber != ethEvent.Raw.BlockNumber {
		el.logger.Info().
			Uint64("block_number", el.state.currentBlockNumber).
			Uint64("new_block_number", ethEvent.Raw.BlockNumber).
			Msg("finished processing events from block")
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

func (el *EventListener) convertEvent(ethEvent *L1MessageSent) *Event {
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
	return event
}

// should be called only from eventProcessor
func (el *EventListener) onNewBlockBegan(ctx context.Context, newBlockInfo *L1MessageSent) error {
	// save previous not empty block info to the database
	if el.state.currentBlockNumber != 0 {
		procBlk := &ProcessedBlock{
			BlockNumber: el.state.currentBlockNumber,
			BlockHash:   el.state.currentBlockHash,
		}

		if err := el.eventStorage.SetLastProcessedBlock(ctx, procBlk); err != nil {
			return err
		}
	}

	el.setCurrentProcessingBlock(newBlockInfo.Raw.BlockNumber, newBlockInfo.Raw.BlockHash)

	return nil
}

func (el *EventListener) setCurrentProcessingBlock(number uint64, hash ethcommon.Hash) {
	el.state.currentBlockNumber = number
	el.state.currentBlockHash = hash
}
