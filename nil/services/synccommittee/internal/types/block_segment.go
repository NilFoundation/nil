package types

import (
	"fmt"

	"github.com/NilFoundation/nil/nil/internal/types"
)

// ShardChainSegment represents a sequence of blocks in a shard
type ShardChainSegment []*Block

var EmptyChainSegment = make(ShardChainSegment, 0)

func NewChainSegment(blocks ...*Block) (ShardChainSegment, error) {
	if err := validateSegment(blocks); err != nil {
		return nil, err
	}

	return blocks, nil
}

func validateSegment(blocks []*Block) error {
	if len(blocks) == 0 {
		return nil
	}

	shardId := blocks[0].ShardId

	for i, block := range blocks {
		if block.ShardId != shardId {
			return fmt.Errorf("shardId mismatch at index %d: %d != %d", i, block.ShardId, shardId)
		}

		if i == 0 {
			continue
		}

		parentRef := BlockToRef(blocks[i-1])
		if err := parentRef.ValidateNext(block); err != nil {
			return fmt.Errorf("parent-child mismatch at index %d: %w", i, err)
		}
	}

	return nil
}

// Concat attempts to append another ShardChainSegment to the current one,
// ensuring sequence validity or returns an error.
func (s ShardChainSegment) Concat(other ShardChainSegment) (ShardChainSegment, error) {
	if len(s) == 0 {
		return other, nil
	}
	if len(other) == 0 {
		return s, nil
	}

	prevLatestRef := BlockToRef(s.Latest())
	nextEarliest := other.Earliest()

	if err := prevLatestRef.ValidateNext(nextEarliest); err != nil {
		return nil, fmt.Errorf("failed to join chain segments: %w", err)
	}

	return append(s, other...), nil
}

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

// ChainSegments represents a mapping of shards to their chain segments
type ChainSegments map[types.ShardId]ShardChainSegment

func NewChainSegments(blocks map[types.ShardId][]*Block) (ChainSegments, error) {
	segments := make(ChainSegments, len(blocks))
	for shardId, blockSlice := range blocks {
		segment, err := NewChainSegment(blockSlice...)
		if err != nil {
			return nil, fmt.Errorf("failed to create chain segment for shard %s: %w", shardId, err)
		}
		segments[shardId] = segment
	}
	return segments, nil
}

// Concat attempts to append another ChainSegments to the current one,
// ensuring sequence validity or returns an error.
//
// Example:
// - [S1] 1 <-- 2 <-- 3                                [S1] 1 <-- 2 <-- 3
// - [S2]       2 <-- 3   +   [S2]       4 <-- 5   =   [S2]       2 <-- 3 <-- 4 <-- 5
// -                          [S3] 3 <-- 4             [S3]             3 <-- 4
func (s ChainSegments) Concat(other ChainSegments) (ChainSegments, error) {
	concatenated := make(ChainSegments, len(s))

	for shardId, segment := range s {
		otherSegment := other[shardId]
		joinedSegment, err := segment.Concat(otherSegment)
		if err != nil {
			return nil, fmt.Errorf("failed to join segments for shard %s: %w", shardId, err)
		}
		concatenated[shardId] = joinedSegment
	}

	for shardId, segment := range other {
		if _, ok := concatenated[shardId]; ok {
			continue
		}
		concatenated[shardId] = segment
	}

	return concatenated, nil
}

func (s ChainSegments) BlocksCount() uint32 {
	sum := uint32(0)
	for _, segment := range s {
		sum += uint32(len(segment))
	}
	return sum
}

// EarliestBlocks returns earliest blocks for each shard
func (s ChainSegments) EarliestBlocks() map[types.ShardId]*Block {
	return s.getEdgeBlocks(false)
}

// LatestBlocks returns latest blocks for each shard
func (s ChainSegments) LatestBlocks() map[types.ShardId]*Block {
	return s.getEdgeBlocks(true)
}

// getEdgeBlocks identifies and returns either the first or last blocks
// for each shard based on the `latest` parameter.
func (s ChainSegments) getEdgeBlocks(latest bool) map[types.ShardId]*Block {
	blocks := make(map[types.ShardId]*Block)

	for shardId, segment := range s {
		if latest {
			blocks[shardId] = segment.Latest()
		} else {
			blocks[shardId] = segment.Earliest()
		}
	}

	return blocks
}
