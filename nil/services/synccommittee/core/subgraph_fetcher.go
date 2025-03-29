package core

import (
	"context"
	"fmt"

	"github.com/NilFoundation/nil/nil/common/logging"
	coreTypes "github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
)

type subgraphFetcher struct {
	rpcClient RpcBlockFetcher
	logger    logging.Logger
}

func newSubgraphFetcher(
	rpcClient RpcBlockFetcher,
	logger logging.Logger,
) *subgraphFetcher {
	return &subgraphFetcher{
		rpcClient: rpcClient,
		logger:    logger,
	}
}

func (f *subgraphFetcher) FetchSubgraph(
	ctx context.Context,
	mainShardBlock *types.Block,
	latestFetched types.BlockRefs,
) (*types.Subgraph, error) {
	segments, err := f.fetchShardChainSegments(ctx, mainShardBlock, latestFetched)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch shard chain segments: %w", err)
	}

	return types.NewSubgraph(mainShardBlock, segments)
}

func (f *subgraphFetcher) fetchShardChainSegments(
	ctx context.Context,
	mainShardBlock *types.Block,
	latestFetched types.BlockRefs,
) (map[coreTypes.ShardId]types.ShardChainSegment, error) {
	childIds, err := types.ChildBlockIds(mainShardBlock)
	if err != nil {
		return nil, err
	}

	segments := make(map[coreTypes.ShardId]types.ShardChainSegment)

	for _, childId := range childIds {
		latestFetchedInShard := latestFetched.TryGet(childId.ShardId)
		segment, err := f.fetchShardChainSegment(ctx, latestFetchedInShard, childId)
		if err != nil {
			return nil, fmt.Errorf("error fetching shard chain segment, childId=%s: %w", childId, err)
		}
		if len(segment) == 0 {
			f.logger.Warn().
				Stringer(logging.FieldShardId, childId.ShardId).
				Stringer(logging.FieldBlockHash, childId.Hash).
				Msg("received empty chain segment, skipping")
			continue
		}

		segments[childId.ShardId] = segment
	}

	return segments, nil
}

func (f *subgraphFetcher) fetchShardChainSegment(
	ctx context.Context,
	latestFetched *types.BlockRef,
	latestInSubgraph types.BlockId,
) (types.ShardChainSegment, error) {
	latestSubBlock, err := f.getBlockById(ctx, latestInSubgraph)
	if err != nil {
		return nil, err
	}

	if err := latestFetched.ValidateDescendant(types.BlockToRef(latestSubBlock)); err != nil {
		return nil, fmt.Errorf("cannot fetch chain segment: %w", err)
	}

	var fetchStartingNumber coreTypes.BlockNumber
	if latestFetched != nil {
		fetchStartingNumber = latestFetched.Number + 1
	} else {
		fetchStartingNumber = 0
	}

	f.logger.Debug().
		Stringer(logging.FieldShardId, latestInSubgraph.ShardId).
		Msgf(
			"Fetching chain segment [%d, %d] from shard %d",
			fetchStartingNumber, latestSubBlock.Number, latestInSubgraph.ShardId,
		)

	const requestBatchSize = 20
	blocks, err := f.rpcClient.GetBlocksRange(
		ctx, latestInSubgraph.ShardId, fetchStartingNumber, latestSubBlock.Number, true, requestBatchSize,
	)
	if err != nil {
		return nil, fmt.Errorf("error fetching chain segment from shard %d: %w", latestInSubgraph.ShardId, err)
	}

	blocks = append(blocks, latestSubBlock)
	return blocks, nil
}

func (f *subgraphFetcher) getBlockById(ctx context.Context, id types.BlockId) (*types.Block, error) {
	latestChild, err := f.rpcClient.GetBlock(ctx, id.ShardId, id.Hash, false)
	if err != nil {
		return nil, fmt.Errorf("error fetching latest child block from shard %d: %w", id.ShardId, err)
	}
	if latestChild == nil {
		return nil, fmt.Errorf(
			"%w: latest child block not found in chain, id=%s: %w", types.ErrBlockNotFound, id, err,
		)
	}
	return latestChild, nil
}
