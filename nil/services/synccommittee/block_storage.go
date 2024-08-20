package synccommittee

import (
	"sync"

	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
)

type BlockStorage struct {
	blocksStorage              map[types.ShardId]map[types.BlockNumber]*jsonrpc.RPCBlock
	lastFetchedBlockPerShard   map[types.ShardId]*jsonrpc.RPCBlock
	lastProvedBlockNumPerShard map[types.ShardId]types.BlockNumber
	mu                         sync.RWMutex // Protects access to the maps
}

func NewBlockStorage() *BlockStorage {
	return &BlockStorage{
		blocksStorage:              make(map[types.ShardId]map[types.BlockNumber]*jsonrpc.RPCBlock),
		lastFetchedBlockPerShard:   make(map[types.ShardId]*jsonrpc.RPCBlock),
		lastProvedBlockNumPerShard: make(map[types.ShardId]types.BlockNumber),
	}
}

func (bs *BlockStorage) GetBlock(shardId types.ShardId, blockNumber types.BlockNumber) *jsonrpc.RPCBlock {
	bs.mu.RLock()
	defer bs.mu.RUnlock()
	if shard, ok := bs.blocksStorage[shardId]; ok {
		return shard[blockNumber]
	}
	return nil
}

func (bs *BlockStorage) SetBlock(shardId types.ShardId, blockNumber types.BlockNumber, block *jsonrpc.RPCBlock) {
	bs.mu.Lock()
	defer bs.mu.Unlock()

	if _, ok := bs.blocksStorage[shardId]; !ok {
		bs.blocksStorage[shardId] = make(map[types.BlockNumber]*jsonrpc.RPCBlock)
	}
	bs.blocksStorage[shardId][blockNumber] = block

	lastFetched := bs.lastFetchedBlockPerShard[shardId]
	if lastFetched == nil || block.Number > lastFetched.Number {
		bs.lastFetchedBlockPerShard[shardId] = block
	}
}

func (bs *BlockStorage) GetLastFetchedBlock(shardId types.ShardId) *jsonrpc.RPCBlock {
	bs.mu.RLock()
	defer bs.mu.RUnlock()
	return bs.lastFetchedBlockPerShard[shardId]
}

func (bs *BlockStorage) GetLastProvedBlockNum(shardId types.ShardId) types.BlockNumber {
	bs.mu.RLock()
	defer bs.mu.RUnlock()
	return bs.lastProvedBlockNumPerShard[shardId]
}

func (bs *BlockStorage) SetLastProvedBlockNum(shardId types.ShardId, blockNum types.BlockNumber) {
	bs.mu.Lock()
	defer bs.mu.Unlock()
	bs.lastProvedBlockNumPerShard[shardId] = blockNum
}

func (bs *BlockStorage) GetBlocksRange(shardId types.ShardId, from, to types.BlockNumber) []*jsonrpc.RPCBlock {
	bs.mu.RLock()
	defer bs.mu.RUnlock()

	shard, ok := bs.blocksStorage[shardId]
	if !ok {
		return nil
	}

	blocks := make([]*jsonrpc.RPCBlock, 0, to-from)
	for i := from; i < to; i++ {
		if block := shard[i]; block != nil {
			blocks = append(blocks, block)
		}
	}
	return blocks
}

func (bs *BlockStorage) CleanupStorage() {
	bs.mu.Lock()
	defer bs.mu.Unlock()

	for shardId, shard := range bs.blocksStorage {
		lastProvedBlockNum := bs.lastProvedBlockNumPerShard[shardId]
		for blockNumber := range shard {
			if blockNumber < lastProvedBlockNum {
				delete(shard, blockNumber)
			}
		}
	}
}
