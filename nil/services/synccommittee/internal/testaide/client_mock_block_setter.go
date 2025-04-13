package testaide

import (
	"context"
	"fmt"
	"iter"
	"log"
	"maps"
	"slices"

	"github.com/NilFoundation/nil/nil/client"
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/types"
	scTypes "github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
)

func ClientMockSetBatches(client *client.ClientMock, batches []*scTypes.BlockBatch) {
	blocksIter := func(yield func(*scTypes.Block) bool) {
		for _, batch := range batches {
			for block := range batch.BlocksIter() {
				if !yield(block) {
					return
				}
			}
		}
	}

	ClientMockSetBlocks(client, blocksIter)
}

func ClientMockSetBlocks(client *client.ClientMock, blocks iter.Seq[*scTypes.Block]) {
	idToBlocks := make(map[scTypes.BlockId]*scTypes.Block)
	shardsToBlocks := make(map[types.ShardId][]*scTypes.Block)

	for block := range blocks {
		blockId := scTypes.IdFromBlock(block)
		if _, ok := idToBlocks[blockId]; ok {
			log.Panicf("block id duplicated in batches: %s", blockId)
		}

		idToBlocks[blockId] = block

		blockSlice := shardsToBlocks[block.ShardId]
		blockSlice = append(blockSlice, block)
		shardsToBlocks[block.ShardId] = blockSlice
	}

	for _, blockSlice := range shardsToBlocks {
		slices.SortFunc(blockSlice, func(a, b *scTypes.Block) int {
			return int(a.Number - b.Number)
		})
	}

	client.GetBlockFunc = func(
		_ context.Context, shardId types.ShardId, blockId any, fullTx bool,
	) (*scTypes.Block, error) {
		if strId, ok := blockId.(string); ok && strId == "latest" {
			shardSlice := shardsToBlocks[shardId]
			if len(shardSlice) == 0 {
				return nil, nil
			}
			return shardSlice[len(shardSlice)-1], nil
		}

		blockHash, ok := blockId.(common.Hash)
		if !ok {
			return nil, fmt.Errorf("unexpected blockId type: %v", blockId)
		}
		id := scTypes.NewBlockId(shardId, blockHash)
		return idToBlocks[id], nil
	}

	client.GetBlocksRangeFunc = func(
		_ context.Context, shardId types.ShardId, from types.BlockNumber, to types.BlockNumber, _ bool, _ int,
	) ([]*scTypes.Block, error) {
		blockRange := make([]*scTypes.Block, 0)
		for _, block := range shardsToBlocks[shardId] {
			if block.Number >= from && block.Number < to {
				blockRange = append(blockRange, block)
			}
		}
		return blockRange, nil
	}

	shardIds := slices.Collect(maps.Keys(shardsToBlocks))
	client.GetShardIdListFunc = func(ctx context.Context) ([]types.ShardId, error) {
		return shardIds, nil
	}
}
