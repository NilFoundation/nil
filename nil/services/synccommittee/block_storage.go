package synccommittee

import (
	"sync"

	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
)

type BlockStorage struct {
	blocksStorage               map[types.ShardId]map[types.BlockNumber]*jsonrpc.RPCBlock
	lastFetchedBlockNumPerShard map[types.ShardId]types.BlockNumber
	lastProvedBlockNumPerShard  map[types.ShardId]types.BlockNumber
	mu                          sync.RWMutex // Protects access to the maps
}

type prunedTransaction struct {
	flags types.MessageFlags
	seqno hexutil.Uint64
	from  types.Address
	to    types.Address
	value types.Value
	data  hexutil.Bytes
}

func NewBlockStorage() *BlockStorage {
	return &BlockStorage{
		blocksStorage:               make(map[types.ShardId]map[types.BlockNumber]*jsonrpc.RPCBlock),
		lastFetchedBlockNumPerShard: make(map[types.ShardId]types.BlockNumber),
		lastProvedBlockNumPerShard:  make(map[types.ShardId]types.BlockNumber),
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

func (bs *BlockStorage) SetBlock(block *jsonrpc.RPCBlock) {
	if block == nil {
		return
	}
	shardId := block.ShardId
	blockNumber := block.Number
	bs.mu.Lock()
	defer bs.mu.Unlock()
	if _, ok := bs.blocksStorage[shardId]; !ok {
		bs.blocksStorage[shardId] = make(map[types.BlockNumber]*jsonrpc.RPCBlock)
	}
	bs.blocksStorage[shardId][blockNumber] = block

	if bs.lastFetchedBlockNumPerShard[shardId] < block.Number {
		bs.lastFetchedBlockNumPerShard[shardId] = block.Number
	}
}

func (bs *BlockStorage) GetLastFetchedBlockNum(shardId types.ShardId) types.BlockNumber {
	bs.mu.RLock()
	defer bs.mu.RUnlock()
	return bs.lastFetchedBlockNumPerShard[shardId]
}

func (bs *BlockStorage) SetLastFetchedBlockNum(shardId types.ShardId, blockNum types.BlockNumber) {
	bs.mu.Lock()
	defer bs.mu.Unlock()
	bs.lastFetchedBlockNumPerShard[shardId] = blockNum
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

	if from < to {
		blocks := make([]*jsonrpc.RPCBlock, 0, to-from)
		for i := from; i < to; i++ {
			if block := shard[i]; block != nil {
				blocks = append(blocks, block)
			}
		}
		return blocks
	}
	return nil
}

func (bs *BlockStorage) GetTransactionsByBlocksRange(shardId types.ShardId, from, to types.BlockNumber) []*prunedTransaction {
	var transactions []*prunedTransaction
	for i := from; i < to; i++ {
		if block := bs.GetBlock(shardId, i); block != nil {
			for _, msg_any := range block.Messages {
				if msg, success := msg_any.(jsonrpc.RPCInMessage); success {
					t := &prunedTransaction{
						flags: msg.Flags,
						seqno: msg.Seqno,
						from:  msg.From,
						to:    msg.To,
						value: msg.Value,
						data:  msg.Data,
					}
					transactions = append(transactions, t)
				}
			}
		}
	}
	return transactions
}

func (bs *BlockStorage) CleanupStorage() {
	bs.mu.Lock()
	defer bs.mu.Unlock()
	for shardId, blocksPerShard := range bs.blocksStorage {
		lastProvedBlockNum, exists := bs.lastProvedBlockNumPerShard[shardId]
		if !exists {
			continue
		}

		for blockNumber := range blocksPerShard {
			if blockNumber < lastProvedBlockNum {
				delete(blocksPerShard, blockNumber)
			}
		}
	}
}
