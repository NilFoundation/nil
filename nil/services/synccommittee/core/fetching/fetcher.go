package fetching

import (
	"context"
	"fmt"
	"iter"
	"slices"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/logging"
	coreTypes "github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
)

const blockRangeBatchSize = 20

type RpcClient interface {
	GetBlock(ctx context.Context, shardId coreTypes.ShardId, blockId any, fullTx bool) (*jsonrpc.RPCBlock, error)
	GetBlocksRange(
		ctx context.Context,
		shardId coreTypes.ShardId,
		from, to coreTypes.BlockNumber,
		fullTx bool,
		batchSize int,
	) ([]*jsonrpc.RPCBlock, error)
	GetShardIdList(ctx context.Context) ([]coreTypes.ShardId, error)
}

type Fetcher struct {
	rpcClient RpcClient
	logger    logging.Logger
}

func NewFetcher(rpcClient RpcClient, logger logging.Logger) *Fetcher {
	return &Fetcher{
		rpcClient: rpcClient,
		logger:    logger,
	}
}

func (f *Fetcher) TryGetBlockByHash(
	ctx context.Context,
	shardId coreTypes.ShardId,
	hash common.Hash,
) (*types.Block, error) {
	block, err := f.rpcClient.GetBlock(ctx, shardId, hash, true)
	if err != nil {
		return nil, fmt.Errorf("error fetching block from shard %d: %w", shardId, err)
	}
	return block, nil
}

func (f *Fetcher) TryGetBlockRef(
	ctx context.Context,
	shardId coreTypes.ShardId,
	hash common.Hash,
) (*types.BlockRef, error) {
	block, err := f.TryGetBlockByHash(ctx, shardId, hash)
	if err != nil {
		return nil, err
	}
	if block == nil {
		return nil, nil
	}
	blockRef := types.BlockToRef(block)
	return &blockRef, nil
}

func (f *Fetcher) GetGenesisBlockRef(
	ctx context.Context,
	shardId coreTypes.ShardId,
) (*types.BlockRef, error) {
	return f.getBlockRefByStrId(ctx, shardId, "earliest")
}

func (f *Fetcher) GetLatestBlockRef(
	ctx context.Context,
	shardId coreTypes.ShardId,
) (*types.BlockRef, error) {
	return f.getBlockRefByStrId(ctx, shardId, "latest")
}

func (f *Fetcher) GetLatestBlock(
	ctx context.Context,
	shardId coreTypes.ShardId,
) (*types.Block, error) {
	return f.getBlockByStrId(ctx, shardId, "latest")
}

func (f *Fetcher) getBlockByStrId(
	ctx context.Context,
	shardId coreTypes.ShardId,
	strId string,
) (*types.Block, error) {
	block, err := f.rpcClient.GetBlock(ctx, shardId, strId, false)
	if err != nil {
		return nil, fmt.Errorf(
			"error fetching block, shardId=%d, id=%s: %w", shardId, strId, err,
		)
	}
	if block == nil {
		return nil, fmt.Errorf(
			"%w: block not found in chain, shardId=%d, id=%s", types.ErrBlockNotFound, shardId, strId,
		)
	}
	return block, nil
}

func (f *Fetcher) getBlockRefByStrId(
	ctx context.Context,
	shardId coreTypes.ShardId,
	strId string,
) (*types.BlockRef, error) {
	block, err := f.getBlockByStrId(ctx, shardId, strId)
	if err != nil {
		return nil, err
	}
	blockRef := types.BlockToRef(block)
	return &blockRef, nil
}

func (f *Fetcher) FetchBlocksSeq(
	ctx context.Context,
	shardId coreTypes.ShardId,
	blocksRange types.BlocksRange,
) iter.Seq2[*types.Block, error] {
	return func(yield func(*types.Block, error) bool) {
		chunks := blocksRange.SplitToChunks(blockRangeBatchSize)

		for _, chunk := range chunks {
			blocks, err := f.fetchChunk(ctx, shardId, chunk)
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

func (f *Fetcher) fetchChunk(
	ctx context.Context,
	shardId coreTypes.ShardId,
	blocksRange types.BlocksRange,
) ([]*types.Block, error) {
	blocks, err := f.rpcClient.GetBlocksRange(
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

func (f *Fetcher) GetShardIdList(ctx context.Context) ([]coreTypes.ShardId, error) {
	shardIds, err := f.rpcClient.GetShardIdList(ctx)
	if err != nil {
		return nil, fmt.Errorf("error fetching shard ids: %w", err)
	}
	shardIds = append(shardIds, coreTypes.MainShardId)
	slices.Sort(shardIds)
	return shardIds, nil
}
