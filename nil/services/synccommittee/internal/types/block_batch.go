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
	Id         BatchId       `json:"id"`
	ParentId   *BatchId      `json:"parentId"`
	Blocks     ChainSegments `json:"blocks"`
	DataProofs DataProofs    `json:"dataProofs"`
	IsSealed   bool          `json:"isSealed"`
	CreatedAt  time.Time     `json:"createdAt"`
	UpdatedAt  time.Time     `json:"updatedAt"`
}

func NewBlockBatch(parentId *BatchId, currentTime time.Time) *BlockBatch {
	return &BlockBatch{
		Id:        NewBatchId(),
		ParentId:  parentId,
		Blocks:    make(ChainSegments),
		CreatedAt: currentTime,
		UpdatedAt: currentTime,
	}
}

func ReconstructExistingBlockBatch(
	id BatchId,
	parentId *BatchId,
	blocks ChainSegments,
	dataProofs DataProofs,
	isSealed bool,
	createdAt time.Time,
	updatedAt time.Time,
) *BlockBatch {
	return &BlockBatch{
		Id:         id,
		ParentId:   parentId,
		Blocks:     blocks,
		DataProofs: dataProofs,
		IsSealed:   isSealed,
		CreatedAt:  createdAt,
		UpdatedAt:  updatedAt,
	}
}

func (b BlockBatch) WithAddedBlocks(segments ChainSegments, currentTime time.Time) (*BlockBatch, error) {
	if b.IsSealed {
		return nil, fmt.Errorf("cannot add blocks to sealed batch with id=%s", b.Id)
	}

	newSegments, err := b.Blocks.Concat(segments)
	if err != nil {
		return nil, fmt.Errorf("failed to add blocks to batch with id=%s: %w", b.Id, err)
	}

	b.Blocks = newSegments
	b.UpdatedAt = currentTime
	return &b, nil
}

func (b BlockBatch) Seal(dataProofs DataProofs, currentTime time.Time) (*BlockBatch, error) {
	if b.IsSealed {
		return nil, fmt.Errorf("batch with id=%s is already sealed", b.Id)
	}

	b.DataProofs = dataProofs
	b.UpdatedAt = currentTime
	b.IsSealed = true

	return &b, nil
}

func (b *BlockBatch) IsEmpty() bool {
	return b.Blocks.BlocksCount() == 0
}

func (b *BlockBatch) BlockIds() []BlockId {
	blockIds := make([]BlockId, 0, b.Blocks.BlocksCount())
	for block := range b.BlocksIter() {
		blockIds = append(blockIds, IdFromBlock(block))
	}
	return blockIds
}

// BlocksIter provides an iterator for traversing over all blocks in the batch
// ordering them by pair (ShardId, BlockNumber)
func (b *BlockBatch) BlocksIter() iter.Seq[*Block] {
	return func(yield func(*Block) bool) {
		sortedShards := slices.Sorted(maps.Keys(b.Blocks))

		for _, shard := range sortedShards {
			segment := b.Blocks[shard]
			for _, block := range segment {
				if !yield(block) {
					return
				}
			}
		}
	}
}

func (b *BlockBatch) EarliestMainBlock() *Block {
	return b.Blocks[types.MainShardId].Earliest()
}

func (b *BlockBatch) LatestMainBlock() *Block {
	return b.Blocks[types.MainShardId].Latest()
}

// ParentRefs returns refs to parent blocks for each shard included in the batch
func (b *BlockBatch) ParentRefs() map[types.ShardId]*BlockRef {
	firstBlocks := b.Blocks.getEdgeBlocks(false)
	refs := make(map[types.ShardId]*BlockRef)
	for shardId, block := range firstBlocks {
		refs[shardId] = GetParentRef(block)
	}
	return refs
}

// EarliestBlocks returns the earliest block for each shard in the batch
func (b *BlockBatch) EarliestBlocks() map[types.ShardId]*Block {
	return b.Blocks.getEdgeBlocks(false)
}

// LatestBlocks returns the latest block for each shard in the batch
func (b *BlockBatch) LatestBlocks() map[types.ShardId]*Block {
	return b.Blocks.getEdgeBlocks(true)
}

// EarliestRefs returns refs to the earliest blocks for each shard in the batch
func (b *BlockBatch) EarliestRefs() BlockRefs {
	latestBlocks := b.EarliestBlocks()
	return BlocksToRefs(latestBlocks)
}

// LatestRefs returns refs to the latest blocks for each shard in the batch
func (b *BlockBatch) LatestRefs() BlockRefs {
	latestBlocks := b.LatestBlocks()
	return BlocksToRefs(latestBlocks)
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
