package types

import (
	"errors"
	"fmt"
	"time"

	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
	"github.com/google/uuid"
)

var (
	ErrBatchNotReady = errors.New("batch is not ready for handling")
	ErrBatchMismatch = errors.New("batch mismatch")
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
	Id             BatchId             `json:"id"`
	ParentId       *BatchId            `json:"parentId"`
	MainShardBlock *jsonrpc.RPCBlock   `json:"mainShardBlock"`
	ChildBlocks    []*jsonrpc.RPCBlock `json:"childBlocks"`
}

func NewBlockBatch(
	parentId *BatchId,
	mainShardBlock *jsonrpc.RPCBlock,
	childBlocks []*jsonrpc.RPCBlock,
) (*BlockBatch, error) {
	if err := validateBatch(mainShardBlock, childBlocks); err != nil {
		return nil, err
	}

	return &BlockBatch{
		Id:             NewBatchId(),
		ParentId:       parentId,
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
			if childHash.Empty() {
				return fmt.Errorf("%w: mainShardBlock.ChildBlocks[%d] is nil", ErrBatchNotReady, i)
			}

			return fmt.Errorf(
				"childBlocks[%d] cannot be nil, mainShardBlock.ChildBlocks[%d] = %s",
				i, i, childHash,
			)
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

func (b *BlockBatch) BlocksCount() uint32 {
	return uint32(len(b.ChildBlocks) + 1)
}

func (b *BlockBatch) AllBlocks() []*jsonrpc.RPCBlock {
	blocks := make([]*jsonrpc.RPCBlock, 0, len(b.ChildBlocks)+1)
	blocks = append(blocks, b.MainShardBlock)
	blocks = append(blocks, b.ChildBlocks...)
	return blocks
}

func (b *BlockBatch) CreateProofTasks(currentTime time.Time) ([]*TaskEntry, error) {
	taskEntries := make([]*TaskEntry, 0, len(b.ChildBlocks)+1)

	aggregateProofsTask := NewAggregateProofsTaskEntry(b.Id, b.MainShardBlock, currentTime)
	taskEntries = append(taskEntries, aggregateProofsTask)

	for _, childBlock := range b.ChildBlocks {
		blockProofTask, err := NewBlockProofTaskEntry(b.Id, aggregateProofsTask, childBlock, currentTime)
		if err != nil {
			return nil, err
		}

		taskEntries = append(taskEntries, blockProofTask)
	}

	return taskEntries, nil
}

type PrunedBatch struct {
	BatchId BatchId
	Blocks  []*PrunedBlock
}

func NewPrunedBatch(batch *BlockBatch) *PrunedBatch {
	out := &PrunedBatch{
		BatchId: batch.Id,
	}
	for _, blk := range batch.ChildBlocks {
		out.Blocks = append(out.Blocks, NewPrunedBlock(blk))
	}
	out.Blocks = append(out.Blocks, NewPrunedBlock(batch.MainShardBlock))
	return out
}
