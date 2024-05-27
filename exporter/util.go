package exporter

import "github.com/NilFoundation/nil/core/types"

func wrapBlockWithShard(id types.ShardId, block *types.Block) *BlockMsg {
	return &BlockMsg{Block: block, Shard: id}
}
