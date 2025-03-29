package types

import (
	"errors"
	"fmt"
	"iter"
	"maps"
	"slices"
	"time"

	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/google/uuid"
)

var (
	ErrBatchNotReady  = errors.New("batch is not ready for handling")
	ErrBatchMismatch  = errors.New("batch mismatch")
	ErrBatchNotProved = errors.New("batch is not proved")
	ErrBlockMismatch  = errors.New("block mismatch")
)

// BatchId Unique ID of a batch of blocks.
type BatchId uuid.UUID

func NewBatchId() BatchId         { return BatchId(uuid.New()) }
func (id BatchId) String() string { return uuid.UUID(id).String() }
func (id BatchId) Bytes() []byte  { return []byte(id.String()) }

// MarshalText implements the encoding.TextMarshaler interface for BatchId.
func (id BatchId) MarshalText() ([]byte, error) {
	uuidValue := uuid.UUID(id)
	return []byte(uuidValue.String()), nil
}

// UnmarshalText implements the encoding.TextUnmarshaler interface for BatchId.
func (id *BatchId) UnmarshalText(data []byte) error {
	uuidValue, err := uuid.Parse(string(data))
	if err != nil {
		return err
	}
	*id = BatchId(uuidValue)
	return nil
}

func (id *BatchId) Type() string {
	return "BatchId"
}

func (id *BatchId) Set(val string) error {
	return id.UnmarshalText([]byte(val))
}

type BlockBatch struct {
	Id        BatchId    `json:"id"`
	ParentId  *BatchId   `json:"parentId"`
	Subgraphs []Subgraph `json:"subgraphs"`
}

func NewBlockBatch(parentId *BatchId, subgraphs ...Subgraph) (*BlockBatch, error) {
	if err := validateBatch(subgraphs); err != nil {
		return nil, err
	}

	return &BlockBatch{
		Id:        NewBatchId(),
		ParentId:  parentId,
		Subgraphs: subgraphs,
	}, nil
}

func validateBatch(subgraphs []Subgraph) error {
	if len(subgraphs) == 0 {
		return errors.New("subgraphs cannot be empty")
	}

	for i, subgraph := range subgraphs {
		if i == 0 {
			continue
		}

		parentMainRef := BlockToRef(subgraphs[i-1].Main)
		if err := parentMainRef.ValidateNext(subgraph.Main); err != nil {
			return fmt.Errorf("parent-child subgraph mismatch at index %d: %w", i, err)
		}
	}

	return nil
}

func (b *BlockBatch) BlocksCount() uint32 {
	sum := uint32(0)
	for _, subgraph := range b.Subgraphs {
		sum += subgraph.BlocksCount()
	}
	return sum
}

func (b *BlockBatch) BlockIds() []BlockId {
	blockIds := make([]BlockId, 0, b.BlocksCount())
	for block := range b.BlocksIter() {
		blockIds = append(blockIds, IdFromBlock(block))
	}
	return blockIds
}

// BlocksIter provides an iterator for traversing over all blocks in the batch
// ordering them by pair (ShardId, BlockNumber)
func (b *BlockBatch) BlocksIter() iter.Seq[*Block] {
	return func(yield func(*Block) bool) {
		for _, subgraph := range b.Subgraphs {
			if !yield(subgraph.Main) {
				return
			}
		}

		for _, shard := range b.sortedExecShards() {
			for _, subgraph := range b.Subgraphs {
				chainSegment := subgraph.Children[shard]
				for _, block := range chainSegment {
					if !yield(block) {
						return
					}
				}
			}
		}
	}
}

func (b *BlockBatch) FirstMainBlock() *Block {
	if len(b.Subgraphs) == 0 {
		return nil
	}
	return b.Subgraphs[0].Main
}

func (b *BlockBatch) LatestMainBlock() *Block {
	if len(b.Subgraphs) == 0 {
		return nil
	}
	return b.Subgraphs[len(b.Subgraphs)-1].Main
}

// ParentRefs returns refs to parent blocks for each shard included in the batch
func (b *BlockBatch) ParentRefs() map[types.ShardId]*BlockRef {
	firstBlocks := b.getEdgeBlocks(false)
	refs := make(map[types.ShardId]*BlockRef)
	for shardId, block := range firstBlocks {
		refs[shardId] = GetParentRef(block)
	}
	return refs
}

// EarliestRefs returns refs to the earliest blocks for each shard in the batch
func (b *BlockBatch) EarliestRefs() BlockRefs {
	return b.getEdgeRefs(false)
}

// LatestRefs returns refs to the latest blocks for each shard in the batch
func (b *BlockBatch) LatestRefs() BlockRefs {
	return b.getEdgeRefs(true)
}

func (b *BlockBatch) getEdgeRefs(latest bool) BlockRefs {
	latestBlocks := b.getEdgeBlocks(latest)
	refs := make(BlockRefs)
	for shardId, block := range latestBlocks {
		refs[shardId] = BlockToRef(block)
	}
	return refs
}

// getEdgeBlocks identifies and returns either the first or last blocks
// for each shard in the batch based on the `latest` parameter.
func (b *BlockBatch) getEdgeBlocks(latest bool) map[types.ShardId]*Block {
	blocks := make(map[types.ShardId]*Block)

	var subgraphsIter iter.Seq2[int, Subgraph]
	if latest {
		subgraphsIter = slices.Backward(b.Subgraphs)
	} else {
		subgraphsIter = slices.All(b.Subgraphs)
	}

	for _, subgraph := range subgraphsIter {
		if _, ok := blocks[subgraph.Main.ShardId]; !ok {
			blocks[subgraph.Main.ShardId] = subgraph.Main
		}
		for shardId, segment := range subgraph.Children {
			if _, ok := blocks[shardId]; ok {
				continue
			}

			if latest {
				blocks[shardId] = segment.Latest()
			} else {
				blocks[shardId] = segment.Earliest()
			}
		}
	}

	return blocks
}

func (b *BlockBatch) sortedExecShards() []types.ShardId {
	shards := make(map[types.ShardId]struct{})

	for _, subgraph := range b.Subgraphs {
		for shard := range subgraph.Children {
			shards[shard] = struct{}{}
		}
	}

	return slices.Sorted(maps.Keys(shards))
}

func (b *BlockBatch) CreateProofTask(currentTime time.Time) (*TaskEntry, error) {
	blockIds := b.BlockIds()
	return NewBatchProofTaskEntry(b.Id, blockIds, currentTime)
}

type PrunedBatch struct {
	BatchId BatchId
	Blocks  []*PrunedBlock
}

func NewPrunedBatch(batch *BlockBatch) *PrunedBatch {
	out := &PrunedBatch{
		BatchId: batch.Id,
	}

	for block := range batch.BlocksIter() {
		out.Blocks = append(out.Blocks, NewPrunedBlock(block))
	}

	return out
}
