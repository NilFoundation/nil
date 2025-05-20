package indexer

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/NilFoundation/nil/nil/client"
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/common/concurrent"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/indexer/driver"
)

const (
	BlockBufferSize = 10000

	maxFetchSize = 500

	timeoutWaitingRpc = 5 * time.Minute
)

type Indexer struct {
	driver      driver.IndexerDriver
	client      client.Client
	allowDbDrop bool

	blocksChan chan *driver.BlockWithShardId

	logger logging.Logger
}

func NewIndexerWithClient(client client.Client) *Indexer {
	return &Indexer{
		client: client,
		logger: logging.NewLogger("indexer"),
	}
}

func StartIndexer(ctx context.Context, cfg *Cfg) error {
	e := &Indexer{
		driver:      cfg.IndexerDriver,
		client:      cfg.Client,
		allowDbDrop: cfg.AllowDbDrop,
		blocksChan:  make(chan *driver.BlockWithShardId, BlockBufferSize),
		logger:      logging.NewLogger("indexer"),
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	if err := e.waitForRpc(ctx, timeoutWaitingRpc); err != nil {
		return fmt.Errorf("failed to wait for rpc node: %w", err)
	}

	shards, err := e.setup(ctx)
	if err != nil {
		return fmt.Errorf("failed to setup indexer: %w", err)
	}

	workers := make([]concurrent.Task, 0, len(shards)+2)
	for i, shard := range shards {
		workers = append(workers, concurrent.MakeTask(
			fmt.Sprintf("[%d] fetcher", i),
			func(ctx context.Context) error {
				return e.startFetchers(ctx, shard)
			}))
	}
	workers = append(workers,
		concurrent.MakeTask("driver export", e.startDriverIndex),
		concurrent.MakeTask("version check", e.runVersionCheckLoop),
	)

	if cfg.DoIndexTxpool {
		workers = append(workers, concurrent.MakeTask("txpool indexer", e.runTxPoolFetcher))
	}

	e.logger.Info().Msg("Starting indexer...")

	return concurrent.Run(ctx, workers...)
}

func (e *Indexer) waitForRpc(ctx context.Context, timeout time.Duration) error {
	notFirstTry := false
	return common.WaitFor(ctx, timeout, 1*time.Second, func(ctx context.Context) bool {
		version, err := e.client.ClientVersion(ctx)
		res := version != "" && err == nil
		if !res && !notFirstTry {
			e.logger.Warn().Err(err).Msg("RPC is not ready, waiting...")
			notFirstTry = true
		}
		return res
	})
}

func (e *Indexer) setup(ctx context.Context) ([]types.ShardId, error) {
	remoteVersion, err := e.readVersionFromClient(ctx)
	if err != nil {
		return nil, err
	}

	localVersion, err := e.driver.FetchVersion(ctx)
	if err != nil {
		return nil, err
	}

	if localVersion != remoteVersion {
		if localVersion.Empty() {
			e.logger.Info().Msg("Database is empty. Recreating...")
		} else {
			if !e.allowDbDrop {
				return nil, fmt.Errorf("version mismatch: blockchain %x, indexer %x", remoteVersion, localVersion)
			}

			e.logger.Info().Msgf("Version mismatch: blockchain %x, indexer %x. Dropping database...",
				remoteVersion, localVersion)
		}

		if err := e.driver.ResetDB(ctx); err != nil {
			return nil, err
		}
	}

	shards, err := e.FetchShards(ctx)
	if err != nil {
		return nil, err
	}

	return append(shards, types.MainShardId), nil
}

func (e *Indexer) readVersionFromClient(ctx context.Context) (common.Hash, error) {
	b, err := e.client.GetBlock(ctx, types.MainShardId, 0, false)
	if err != nil {
		return common.EmptyHash, fmt.Errorf("failed to get genesis block from main shard: %w", err)
	}
	return b.Hash, nil
}

func (e *Indexer) startFetchers(ctx context.Context, shardId types.ShardId) error {
	logger := e.logger.With().Stringer(logging.FieldShardId, shardId).Logger()

	lastProcessedBlock, err := concurrent.RunWithRetries(ctx, 1*time.Second, 10, func() (*types.BlockNumber, error) {
		return e.driver.FetchLatestProcessedBlockId(ctx, shardId)
	})
	if err != nil {
		return fmt.Errorf("failed to fetch last processed block id: %w", err)
	}

	// If the db is empty, add the top block to the queue.
	if *lastProcessedBlock == types.InvalidBlockNumber {
		topBlock, err := concurrent.RunWithRetries(
			ctx,
			1*time.Second,
			10,
			func() (*types.BlockWithExtractedData, error) {
				return e.FetchBlock(ctx, shardId, "latest")
			})
		if err != nil {
			return fmt.Errorf("failed to fetch last block: %w", err)
		}

		logger.Info().Msgf("No blocks processed yet. Adding the top block %d...", topBlock.Id)
		e.blocksChan <- &driver.BlockWithShardId{BlockWithExtractedData: topBlock, ShardId: shardId}
		lastProcessedBlock = &topBlock.Id
	}

	return concurrent.Run(ctx,
		concurrent.MakeTask(
			fmt.Sprintf("[%d] top fetcher", shardId),
			func(ctx context.Context) error {
				return e.runTopFetcher(ctx, shardId, *lastProcessedBlock+1)
			}),
		concurrent.MakeTask(
			fmt.Sprintf("[%d] bottom fetcher", shardId),
			func(ctx context.Context) error {
				return e.runBottomFetcher(ctx, shardId, *lastProcessedBlock)
			}),
	)
}

func (e *Indexer) runVersionCheckLoop(ctx context.Context) error {
	// Wait (if needed, e.g., starting from scratch) for the genesis block to be fetched.
	version, err := concurrent.RunWithRetries(ctx, 1*time.Second, 10, func() (common.Hash, error) {
		version, err := e.driver.FetchVersion(ctx)
		if err != nil {
			return common.EmptyHash, err
		}
		if version.Empty() {
			// this will cause the retry, not the error
			return common.EmptyHash, errors.New("version is empty")
		}
		return version, nil
	})
	if err != nil {
		return fmt.Errorf("failed to fetch local version: %w", err)
	}

	e.logger.Info().Msgf("Running version check loop. Local version: %s", version)

	return concurrent.RunTickerLoopWithErr(ctx, 10*time.Second, func(ctx context.Context) error {
		remoteVersion, err := e.readVersionFromClient(ctx)
		if err != nil {
			e.logger.Warn().Err(err).Msg("Failed to fetch remote version")
			return nil
		}

		if version != remoteVersion {
			return fmt.Errorf("local version is outdated; local: %s, remote: %s", version, remoteVersion)
		}
		return nil
	})
}

func (e *Indexer) pushBlocks(
	ctx context.Context,
	shardId types.ShardId,
	fromId types.BlockNumber,
	toId types.BlockNumber,
) (types.BlockNumber, error) {
	const batchSize = 10
	for id := fromId; id < toId; id += batchSize {
		batchEndId := id + batchSize
		if batchEndId > toId {
			batchEndId = toId
		}
		blocks, err := e.FetchBlocks(ctx, shardId, id, batchEndId)
		if err != nil {
			return id, err
		}
		for _, b := range blocks {
			e.blocksChan <- &driver.BlockWithShardId{BlockWithExtractedData: b, ShardId: shardId}
		}
	}
	return toId, nil
}

// runTopFetcher fetches blocks from `from` and indefinitely.
func (e *Indexer) runTopFetcher(ctx context.Context, shardId types.ShardId, from types.BlockNumber) error {
	logger := e.logger.With().Stringer(logging.FieldShardId, shardId).Logger()
	logger.Info().Msgf("Starting top fetcher from %d", from)

	const fetchInterval = 1 * time.Second
	skippedTicks := 0
	const skippedTicksReportPeriod = 5
	concurrent.RunTickerLoop(ctx, fetchInterval, func(ctx context.Context) {
		topBlock, err := e.FetchBlock(ctx, shardId, "latest")
		if err != nil {
			logger.Warn().Err(err).Msg("Failed to fetch latest block")
			return
		}

		// totally synced on top level
		if topBlock.Id < from {
			skippedTicks++
			if skippedTicks%skippedTicksReportPeriod == 0 {
				logger.Warn().Msgf("No new blocks for %s, remote top block %d, local top block %d",
					time.Duration(skippedTicks)*fetchInterval, topBlock.Id, from)
			}
			return
		}
		skippedTicks = 0

		next := min(topBlock.Id, from+maxFetchSize)
		logger.Info().Msgf("Fetching blocks from %d to %d", from, next)
		from, err = e.pushBlocks(ctx, shardId, from, next)
		if err != nil {
			logger.Error().Err(err).Msg("Failed to fetch blocks")
			return
		}

		if from == topBlock.Id {
			e.blocksChan <- &driver.BlockWithShardId{BlockWithExtractedData: topBlock, ShardId: shardId}
			from++
		}
	})

	return nil
}

// runBottomFetcher fetches blocks from the earliest absent block up to the `to`.
func (e *Indexer) runBottomFetcher(ctx context.Context, shardId types.ShardId, to types.BlockNumber) error {
	logger := e.logger.With().Stringer(logging.FieldShardId, shardId).Logger()

	if to == 0 {
		logger.Info().Msg("Bottom fetcher has no blocks to fetch")
		return nil
	}

	from, err := concurrent.RunWithRetries(ctx, 1*time.Second, 10, func() (types.BlockNumber, error) {
		return e.driver.FetchEarliestAbsentBlockId(ctx, shardId)
	})
	if err != nil {
		return fmt.Errorf("failed to fetch earliest absent block: %w", err)
	}

	// from > to in two cases: there is no genesis block, or all blocks till to are present.
	if from > to {
		haveGenesisBlock, err := concurrent.RunWithRetries(ctx, 1*time.Second, 10, func() (bool, error) {
			return e.driver.HaveBlock(ctx, shardId, 0)
		})
		if err != nil {
			return fmt.Errorf("failed to check if we have the top block: %w", err)
		}
		if haveGenesisBlock {
			logger.Info().Msg("Bottom fetcher has no blocks to fetch")
			return nil
		}
		from = 0
	}

	upTo, err := concurrent.RunWithRetries(ctx, 1*time.Second, 10, func() (types.BlockNumber, error) {
		return e.driver.FetchNextPresentBlockId(ctx, shardId, from)
	})
	if err != nil {
		return fmt.Errorf("failed to fetch next present block id: %w", err)
	}
	check.PanicIfNot(upTo <= to)
	to = upTo

	logger.Info().Msgf("Starting bottom fetcher from %d to %d", from, to)

	if err := concurrent.RunTickerLoopWithErr(ctx, 1*time.Second, func(ctx context.Context) error {
		next := min(from+maxFetchSize, to)
		if from == next {
			return concurrent.ErrStopIteration
		}

		logger.Debug().Msgf("Fetching blocks from %d to %d", from, next)
		from, err = e.pushBlocks(ctx, shardId, from, next)
		if err != nil {
			logger.Error().Err(err).Msg("Failed to fetch blocks")
		}
		return nil
	}); err != nil {
		return fmt.Errorf("failed to fetch blocks: %w", err)
	}

	logger.Info().Msgf("Bottom fetcher finished fetching blocks up to %d", to)
	return nil
}

func (e *Indexer) runTxPoolFetcher(ctx context.Context) error {
	numShards := uint64(0)
	txPoolChan := make(chan []*driver.TxPoolStatus, 10000)
	defer close(txPoolChan)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case txs := <-txPoolChan:
				if err := e.driver.IndexTxPool(ctx, txs); err != nil {
					e.logger.Error().Err(err).Msg("Failed to export tx pool")
					continue
				}
			}
		}
	}()

	numShards, err := e.client.GetNumShards(ctx)
	if err != nil {
		return fmt.Errorf("failed to get number of shards: %w", err)
	}

	concurrent.RunTickerLoop(ctx, 1*time.Second, func(ctx context.Context) {
		statuses := make([]*driver.TxPoolStatus, 0, numShards)
		for i := range numShards {
			shardId := types.ShardId(i)
			txPool, err := e.client.GetTxpoolStatus(ctx, shardId)
			if err != nil {
				e.logger.Error().Err(err).
					Stringer(logging.FieldShardId, shardId).
					Msg("Failed to get tx pool status")
				continue
			}
			tx := &driver.TxPoolStatus{
				TxPoolStatus: txPool,
				ShardId:      shardId,
				Timestamp:    time.Now(),
			}
			statuses = append(statuses, tx)
		}
		e.logger.Info().Msgf("Fetched tx pool statuses for %d shards", len(statuses))
		txPoolChan <- statuses
	})
	return nil
}

func (e *Indexer) startDriverIndex(ctx context.Context) error {
	e.logger.Info().Msg("Starting driver export...")

	var blockBuffer []*driver.BlockWithShardId
	concurrent.RunTickerLoop(ctx, 1*time.Second, func(ctx context.Context) {
		// read available blocks
		for len(e.blocksChan) > 0 && len(blockBuffer) < BlockBufferSize {
			blockBuffer = append(blockBuffer, <-e.blocksChan)
		}

		if len(blockBuffer) == 0 {
			return
		}

		if err := e.driver.IndexBlocks(ctx, blockBuffer); err != nil {
			e.logger.Error().Err(err).Msg("Failed to export blocks; will retry in the next round.")
			return
		}
		blockBuffer = blockBuffer[:0]
	})

	return nil
}
