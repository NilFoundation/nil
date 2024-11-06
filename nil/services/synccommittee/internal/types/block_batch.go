package types

import (
	"errors"
	"fmt"

	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
)

type BlockBatch struct {
	Id             BatchId             `json:"id"`
	MainShardBlock *jsonrpc.RPCBlock   `json:"mainShardBlock"`
	ChildBlocks    []*jsonrpc.RPCBlock `json:"childBlocks"`
}

func NewBlockBatch(mainShardBlock *jsonrpc.RPCBlock, childBlocks []*jsonrpc.RPCBlock) (*BlockBatch, error) {
	if err := validateBatch(mainShardBlock, childBlocks); err != nil {
		return nil, err
	}

	return &BlockBatch{
		Id:             NewBatchId(),
		MainShardBlock: mainShardBlock,
		ChildBlocks:    childBlocks,
	}, nil
}

func validateBatch(mainShardBlock *jsonrpc.RPCBlock, childBlocks []*jsonrpc.RPCBlock) error {
	switch {
	case mainShardBlock == nil:
		return errors.New("mainShardBlock cannot be nil")

	case childBlocks == nil:
		return errors.New("childBlocks cannot be nil")

	case mainShardBlock.ShardId != types.MainShardId:
		return fmt.Errorf("mainShardBlock is not from the main shard: %d", mainShardBlock.ShardId)

	case len(childBlocks) != len(mainShardBlock.ChildBlocks):
		return fmt.Errorf(
			"childBlocks and mainShardBlock.ChildBlocks have different length: %d != %d",
			len(childBlocks), len(mainShardBlock.ChildBlocks),
		)
	}

	for i, childHash := range mainShardBlock.ChildBlocks {
		child := childBlocks[i]
		if child == nil {
			return fmt.Errorf("childBlocks[%d] cannot be nil", i)
		}

		if childHash != child.Hash {
			return fmt.Errorf(
				"childBlocks[%d].Hash != mainShardBlock.ChildBlocks[%d]: %s != %s",
				i, i, childHash, childBlocks[i].Hash,
			)
		}
	}
	return nil
}

func (b *BlockBatch) AllBlocks() []*jsonrpc.RPCBlock {
	blocks := make([]*jsonrpc.RPCBlock, 0, len(b.ChildBlocks)+1)
	blocks = append(blocks, b.MainShardBlock)
	blocks = append(blocks, b.ChildBlocks...)
	return blocks
}

func (b *BlockBatch) CreateProofTasks() []*TaskEntry {
	taskEntries := make([]*TaskEntry, 0, len(b.ChildBlocks)+1)

	aggregateProofsTask := NewAggregateProofsTaskEntry(b.Id, b.MainShardBlock)
	taskEntries = append(taskEntries, aggregateProofsTask)

	for _, childBlock := range b.ChildBlocks {
		blockProofTask := NewBlockProofTaskEntry(b.Id, aggregateProofsTask.Task.Id, childBlock.Hash)
		taskEntries = append(taskEntries, blockProofTask)
	}

	return taskEntries
}
