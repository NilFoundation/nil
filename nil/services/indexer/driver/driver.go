package driver

import (
	"context"
	types2 "github.com/NilFoundation/nil/nil/services/indexer/types"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/types"
)

type IndexerDriver interface {
	FetchBlock(context.Context, types.ShardId, types.BlockNumber) (*types.Block, error)
	FetchLatestProcessedBlockId(context.Context, types.ShardId) (*types.BlockNumber, error)
	FetchEarliestAbsentBlockId(context.Context, types.ShardId) (types.BlockNumber, error)
	FetchNextPresentBlockId(context.Context, types.ShardId, types.BlockNumber) (types.BlockNumber, error)
	FetchAddressActions(types.Address, db.Timestamp) ([]types2.AddressAction, error)
	SetupScheme(ctx context.Context, params SetupParams) error
	IndexBlocks(context.Context, []*BlockWithShardId) error
	HaveBlock(context.Context, types.ShardId, types.BlockNumber) (bool, error)
}

type BlockWithShardId struct {
	*types.BlockWithExtractedData
	ShardId types.ShardId
}

type SetupParams struct {
	AllowDbDrop bool
	// Version is the hash of the genesis block of the main shard (must become more complex later).
	Version common.Hash
}
