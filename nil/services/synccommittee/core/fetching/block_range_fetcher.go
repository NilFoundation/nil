package fetching

import (
	"context"
	"fmt"
	"iter"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/logging"
	coreTypes "github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
)

const blockRangeBatchSize = 20

type fetcher struct {
	rpcClient RpcBlockFetcher
	logger    logging.Logger
}

func newFetcher(rpcClient RpcBlockFetcher, logger logging.Logger) *fetcher {
	return &fetcher{
		rpcClient: rpcClient,
		logger:    logger,
	}
}

func (bf *fetcher) GetBlockRef(
	ctx context.Context,
	shardId coreTypes.ShardId,
	hash common.Hash,
) (*types.BlockRef, error) {
	block, err := bf.rpcClient.GetBlock(ctx, shardId, hash, true)
	if err != nil {
		return nil, fmt.Errorf("error fetching block from shard %d: %w", shardId, err)
	}
	if block == nil {
		return nil, fmt.Errorf("block not found in shard %d: %s", shardId, hash)
	}
	blockRef := types.BlockToRef(block)
	return &blockRef, nil
}

func (bf *fetcher) GetLatestBlockRef(
	ctx context.Context,
	shardId coreTypes.ShardId,
) (*types.BlockRef, error) {
	block, err := bf.rpcClient.GetBlock(ctx, shardId, "latest", false)
	if err != nil {
		return nil, fmt.Errorf("error fetching latest block, shardId=%d: %w", shardId, err)
	}
	if block == nil {
		return nil, fmt.Errorf("%w: latest block not found in chain, shardId=%d", types.ErrBlockNotFound, shardId)
	}
	blockRef := types.BlockToRef(block)
	return &blockRef, nil
}

func (bf *fetcher) FetchBlocksSeq(
	ctx context.Context,
	shardId coreTypes.ShardId,
	blocksRange types.BlocksRange,
) iter.Seq2[*types.Block, error] {
	return func(yield func(*types.Block, error) bool) {
		chunks := blocksRange.SplitToChunks(blockRangeBatchSize)

		for _, chunk := range chunks {
			blocks, err := bf.fetchChunk(ctx, shardId, chunk)
			if err != nil {
				yield(nil, err)
				return
			}
			for _, block := range blocks {
				if !yield(block, nil) {
					return
				}
			}
		}
	}
}

func (bf *fetcher) fetchChunk(
	ctx context.Context,
	shardId coreTypes.ShardId,
	blocksRange types.BlocksRange,
) ([]*types.Block, error) {
	blocks, err := bf.rpcClient.GetBlocksRange(
		ctx, shardId, blocksRange.Start, blocksRange.End+1, true, blockRangeBatchSize,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"error fetching blocks from shard %d in range [%d, %d]: %w",
			shardId, blocksRange.Start, blocksRange.End, err,
		)
	}

	return blocks, nil
}
