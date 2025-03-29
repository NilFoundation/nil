package types

import (
	"errors"
	"fmt"

	"github.com/NilFoundation/nil/nil/internal/types"
)

// ShardChainSegment represents a sequence of blocks in a shard
type ShardChainSegment []*Block

// Earliest retrieves the first block in the segment
func (s ShardChainSegment) Earliest() *Block {
	if len(s) == 0 {
		return nil
	}
	return s[0]
}

// Latest retrieves the last block in the segment
func (s ShardChainSegment) Latest() *Block {
	if len(s) == 0 {
		return nil
	}
	return s[len(s)-1]
}

// Subgraph represents a structure containing a main blockchain segment and its associated exec shard blocks
type Subgraph struct {
	Main     *Block
	Children map[types.ShardId]ShardChainSegment
}

func NewSubgraph(main *Block, children map[types.ShardId]ShardChainSegment) (*Subgraph, error) {
	if err := validateSubgraph(main, children); err != nil {
		return nil, err
	}

	return &Subgraph{
		Main:     main,
		Children: children,
	}, nil
}

func (s *Subgraph) BlocksCount() uint32 {
	res := uint32(1) // single main block
	for _, segment := range s.Children {
		res += uint32(len(segment))
	}
	return res
}

func validateSubgraph(mainShardBlock *Block, children map[types.ShardId]ShardChainSegment) error {
	switch {
	case mainShardBlock == nil:
		return errors.New("mainShardBlock cannot be nil")

	case mainShardBlock.ShardId != types.MainShardId:
		return fmt.Errorf("mainShardBlock is not from the mainShardBlock shard: %d", mainShardBlock.ShardId)
	}

	for shardId, segment := range children {
		if err := validateSegment(segment); err != nil {
			return fmt.Errorf("segment validation failed for shard %d: %w", shardId, err)
		}
	}

	return nil
}

func validateSegment(segment ShardChainSegment) error {
	if len(segment) == 0 {
		return errors.New("segment cannot be empty")
	}

	shardId := segment[0].ShardId

	for i, block := range segment {
		if block.ShardId != shardId {
			return fmt.Errorf("shardId mismatch at index %d: %d != %d", i, block.ShardId, shardId)
		}

		if i == 0 {
			continue
		}

		parentRef := BlockToRef(segment[i-1])
		if err := parentRef.ValidateNext(block); err != nil {
			return fmt.Errorf("parent-child mismatch at index %d: %w", i, err)
		}
	}

	return nil
}
