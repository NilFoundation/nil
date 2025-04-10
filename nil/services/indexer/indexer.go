package indexer

import (
	"context"
	"fmt"
	"sync/atomic"
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
	BlockBufferSize     = 10000
	InitialRoundsAmount = 1000

	maxFetchSize = 500

	timeoutWaitingRpc = 5 * time.Minute
)

type Indexer struct {
	driver      driver.IndexerDriver
	client      client.Client
	allowDbDrop bool

	blocksChan chan *driver.BlockWithShardId
	indexRound atomic.Uint32
}

func NewIndexerWithClient(client client.Client) *Indexer {
	return &Indexer{
		client: client,
	}
}

func StartIndexer(ctx context.Context, cfg *Cfg) error {
	logger.Info().Msg("Starting indexer...")

	e := &Indexer{
		driver:      cfg.IndexerDriver,
		client:      cfg.Client,
		allowDbDrop: cfg.AllowDbDrop,
		blocksChan:  make(chan *driver.BlockWithShardId, BlockBufferSize),
	}

	if err := e.waitForRpc(ctx, timeoutWaitingRpc); err != nil {
		return fmt.Errorf("failed to wait for rpc node: %w", err)
	}

	shards, err := e.setup(ctx)
	if err != nil {
		return fmt.Errorf("failed to setup indexer: %w", err)
	}

	workers := make([]concurrent.Task, 0, len(shards)+1)
	for i, shard := range shards {
		workers = append(workers, concurrent.MakeTask(
			fmt.Sprintf("[%d] fetcher", i),
			func(ctx context.Context) error {
				return e.startFetchers(ctx, shard)
			}))
	}
	workers = append(workers, concurrent.MakeTask(
		"driver export",
		func(ctx context.Context) error {
			return e.startDriverIndex(ctx)
		}))

	if cfg.DoIndexTxpool {
		workers = append(workers, concurrent.MakeTask("txpool indexer", func(ctx context.Context) error {
			return e.runTxPoolFetcher(ctx)
		}))
	}

	return concurrent.Run(ctx, workers...)
}

func (e *Indexer) waitForRpc(ctx context.Context, timeout time.Duration) error {
	notFirstTry := false
	return common.WaitFor(ctx, timeout, 1*time.Second, func(ctx context.Context) bool {
		version, err := e.client.ClientVersion(ctx)
		res := version != "" && err == nil
		if !res && !notFirstTry {
			logger.Warn().Err(err).Msg("RPC is not ready, waiting...")
			notFirstTry = true
		}
		return res
	})
}

func (e *Indexer) setup(ctx context.Context) ([]types.ShardId, error) {
	version, err := e.readVersionFromClient(ctx)
	if err != nil {
		return nil, err
	}
	if err := e.driver.SetupScheme(ctx, driver.SetupParams{
		AllowDbDrop: e.allowDbDrop,
		Version:     version,
	}); err != nil {
		return nil, err
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
	logger := logger.With().Stringer(logging.FieldShardId, shardId).Logger()
	logger.Info().Msg("Starting fetchers...")

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
	logger := logger.With().Stringer(logging.FieldShardId, shardId).Logger()
	logger.Info().Msgf("Starting top fetcher from %d", from)

	ticker := time.NewTicker(1 * time.Second)
	curExportRound := e.indexRound.Load() + InitialRoundsAmount
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			newExportRound := e.indexRound.Load()
			if curExportRound == newExportRound {
				continue
			}

			topBlock, err := e.FetchBlock(ctx, shardId, "latest")
			if err != nil {
				logger.Error().Err(err).Msg("Failed to fetch latest block")
				continue
			}

			// totally synced on top level
			if topBlock.Id < from {
				continue
			}

			next := min(topBlock.Id, from+maxFetchSize)
			logger.Info().Msgf("Fetching blocks from %d to %d", from, next)
			from, err = e.pushBlocks(ctx, shardId, from, next)
			if err != nil {
				logger.Error().Err(err).Msg("Failed to fetch blocks")
				continue
			}

			if from == topBlock.Id {
				e.blocksChan <- &driver.BlockWithShardId{BlockWithExtractedData: topBlock, ShardId: shardId}
				from++
			}

			curExportRound = newExportRound
		}
	}
}

// runBottomFetcher fetches blocks from the earliest absent block up to the `to`.
func (e *Indexer) runBottomFetcher(ctx context.Context, shardId types.ShardId, to types.BlockNumber) error {
	logger := logger.With().Stringer(logging.FieldShardId, shardId).Logger()

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

	next, err := concurrent.RunWithRetries(ctx, 1*time.Second, 10, func() (types.BlockNumber, error) {
		return e.driver.FetchNextPresentBlockId(ctx, shardId, from)
	})
	if err != nil {
		return fmt.Errorf("failed to fetch next present block id: %w", err)
	}
	check.PanicIfNot(next <= to)
	to = next

	logger.Info().Msgf("Starting bottom fetcher from %d to %d", from, to)

	ticker := time.NewTicker(1 * time.Second)
	curExportRound := e.indexRound.Load() + InitialRoundsAmount
	for from < to {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			newExportRound := e.indexRound.Load()
			if curExportRound == newExportRound {
				continue
			}

			next := min(next+maxFetchSize, to)
			logger.Info().Msgf("Fetching blocks from %d to %d", from, next)
			from, err = e.pushBlocks(ctx, shardId, from, next)
			if err != nil {
				logger.Error().Err(err).Msg("Failed to fetch blocks")
				continue
			}
			curExportRound = newExportRound
		}
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
					logger.Error().Err(err).Msg("Failed to export tx pool")
					continue
				}
			}
		}
	}()

	numShards, err := e.client.GetNumShards(ctx)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to get number of shards")
	}

	concurrent.RunTickerLoop(ctx, 1*time.Second, func(ctx context.Context) {
		statuses := make([]*driver.TxPoolStatus, 0, numShards)
		for shardId := range numShards {
			txPool, err := e.client.GetTxpoolStatus(ctx, types.ShardId(shardId))
			if err != nil {
				logger.Error().Err(err).Int(logging.FieldShardId, int(shardId)).Msg("Failed to get tx pool status")
				continue
			}
			tx := &driver.TxPoolStatus{
				TxPoolStatus: txPool,
				ShardId:      types.ShardId(shardId),
				Timestamp:    time.Now(),
			}
			statuses = append(statuses, tx)
		}
		logger.Info().Msgf("Fetched tx pool statuses for %d shards", len(statuses))
		txPoolChan <- statuses
	})
	return nil
}

func (e *Indexer) startDriverIndex(ctx context.Context) error {
	logger.Info().Msg("Starting driver export...")

	ticker := time.NewTicker(1 * time.Second)
	var blockBuffer []*driver.BlockWithShardId
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			// read available blocks
			for len(e.blocksChan) > 0 && len(blockBuffer) < BlockBufferSize {
				blockBuffer = append(blockBuffer, <-e.blocksChan)
			}

			if len(blockBuffer) == 0 {
				continue
			}

			if err := e.driver.IndexBlocks(ctx, blockBuffer); err != nil {
				logger.Error().Err(err).Msg("Failed to export blocks; will retry in the next round.")
				continue
			}
			blockBuffer = blockBuffer[:0]
			e.incrementRound()
		}
	}
}

func (e *Indexer) incrementRound() {
	e.indexRound.CompareAndSwap(100000, 0)
	e.indexRound.Add(1)
}
