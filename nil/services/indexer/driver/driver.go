package driver

import (
	"context"
	"time"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/types"
	indexertypes "github.com/NilFoundation/nil/nil/services/indexer/types"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
)

type IndexerDriver interface {
	FetchBlock(context.Context, types.ShardId, types.BlockNumber) (*types.Block, error)
	FetchLatestProcessedBlockId(context.Context, types.ShardId) (*types.BlockNumber, error)
	FetchEarliestAbsentBlockId(context.Context, types.ShardId) (types.BlockNumber, error)
	FetchNextPresentBlockId(context.Context, types.ShardId, types.BlockNumber) (types.BlockNumber, error)
	FetchAddressActions(context.Context, types.Address, types.BlockNumber) ([]indexertypes.AddressAction, error)
	SetupScheme(ctx context.Context, params SetupParams) error
	IndexBlocks(context.Context, []*BlockWithShardId) error
	IndexTxPool(context.Context, []*TxPoolStatus) error
	HaveBlock(context.Context, types.ShardId, types.BlockNumber) (bool, error)
}

type BlockWithShardId struct {
	*types.BlockWithExtractedData
	ShardId types.ShardId `json:"shardId"`
}

type SetupParams struct {
	AllowDbDrop bool
	// Version is the hash of the genesis block of the main shard (must become more complex later).
	Version common.Hash
}

type TxPoolStatus struct {
	jsonrpc.TxPoolStatus
	ShardId   types.ShardId `ch:"shard_id"`
	Timestamp time.Time     `ch:"timestamp"`
}
