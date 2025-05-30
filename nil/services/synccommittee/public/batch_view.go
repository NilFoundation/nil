package public

import (
	"time"

	"github.com/NilFoundation/nil/nil/common"
	coreTypes "github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
)

type (
	BlockRef              = types.BlockRef
	BlockRefs             = types.BlockRefs
	ShardChainSegmentView = []common.Hash
	ChainSegmentsView     = map[coreTypes.ShardId]ShardChainSegmentView
)

type batchViewCommon struct {
	Id        BatchId   `json:"id"`
	ParentId  *BatchId  `json:"parentId"`
	IsSealed  bool      `json:"isSealed"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func newBatchViewCommon(
	id BatchId,
	parentId *BatchId,
	isSealed bool,
	createdAt time.Time,
	updatedAt time.Time,
) batchViewCommon {
	return batchViewCommon{
		Id:        id,
		ParentId:  parentId,
		IsSealed:  isSealed,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}
}

type BatchViewCompact struct {
	batchViewCommon
	BlocksCount int `json:"blocksCount"`
}

func NewBatchViewCompact(
	id BatchId,
	parentId *BatchId,
	isSealed bool,
	createdAt time.Time,
	updatedAt time.Time,
	blocksCount int,
) *BatchViewCompact {
	return &BatchViewCompact{
		batchViewCommon: newBatchViewCommon(
			id,
			parentId,
			isSealed,
			createdAt,
			updatedAt,
		),
		BlocksCount: blocksCount,
	}
}

type BatchViewDetailed struct {
	batchViewCommon
	Blocks ChainSegmentsView `json:"blocks"`
}

func NewBatchViewDetailed(batch *types.BlockBatch) *BatchViewDetailed {
	blocks := make(ChainSegmentsView)
	for shardId, segment := range batch.Blocks {
		view := make(ShardChainSegmentView, 0, len(segment))
		for _, blockId := range segment {
			view = append(view, blockId.Hash)
		}
		blocks[shardId] = view
	}

	return &BatchViewDetailed{
		batchViewCommon: newBatchViewCommon(
			batch.Id,
			batch.ParentId,
			batch.IsSealed,
			batch.CreatedAt,
			batch.UpdatedAt,
		),
		Blocks: blocks,
	}
}
