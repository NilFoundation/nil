package jsonrpc

import (
	"context"
	"fmt"
	"time"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/hexutil"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/execution"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/msgpool"
	"github.com/NilFoundation/nil/rpc/filters"
	"github.com/NilFoundation/nil/rpc/transport"
	"github.com/rs/zerolog"
)

// EthAPI is a collection of functions that are exposed in the
type EthAPI interface {
	// Block related
	GetBlockByNumber(ctx context.Context, shardId types.ShardId, number transport.BlockNumber, fullTx bool) (*RPCBlock, error)
	GetBlockByHash(ctx context.Context, shardId types.ShardId, hash common.Hash, fullTx bool) (*RPCBlock, error)
	GetBlockTransactionCountByNumber(ctx context.Context, shardId types.ShardId, number transport.BlockNumber) (*hexutil.Uint, error)
	GetBlockTransactionCountByHash(ctx context.Context, shardId types.ShardId, hash common.Hash) (*hexutil.Uint, error)

	// Message related
	GetInMessageByHash(ctx context.Context, shardId types.ShardId, hash common.Hash) (*RPCInMessage, error)
	GetInMessageByBlockHashAndIndex(ctx context.Context, shardId types.ShardId, hash common.Hash, index hexutil.Uint64) (*RPCInMessage, error)
	GetInMessageByBlockNumberAndIndex(ctx context.Context, shardId types.ShardId, number transport.BlockNumber, index hexutil.Uint64) (*RPCInMessage, error)
	GetRawInMessageByBlockNumberAndIndex(ctx context.Context, shardId types.ShardId, number transport.BlockNumber, index hexutil.Uint64) (hexutil.Bytes, error)
	GetRawInMessageByBlockHashAndIndex(ctx context.Context, shardId types.ShardId, hash common.Hash, index hexutil.Uint64) (hexutil.Bytes, error)
	GetRawInMessageByHash(ctx context.Context, shardId types.ShardId, hash common.Hash) (hexutil.Bytes, error)

	// Receipt related (see ./eth_receipt.go)
	GetInMessageReceipt(ctx context.Context, shardId types.ShardId, hash common.Hash) (*RPCReceipt, error)

	// Account related
	GetBalance(ctx context.Context, address types.Address, blockNrOrHash transport.BlockNumberOrHash) (*hexutil.Big, error)
	GetTransactionCount(ctx context.Context, address types.Address, blockNrOrHash transport.BlockNumberOrHash) (*hexutil.Uint64, error)
	GetCode(ctx context.Context, address types.Address, blockNrOrHash transport.BlockNumberOrHash) (hexutil.Bytes, error)

	// Sending related
	SendRawTransaction(ctx context.Context, encoded hexutil.Bytes) (common.Hash, error)

	// Logs related
	NewFilter(_ context.Context, query filters.FilterQuery) (string, error)
	NewPendingTransactionFilter(_ context.Context) (string, error)
	NewBlockFilter(_ context.Context) (string, error)
	UninstallFilter(_ context.Context, id string) (isDeleted bool, err error)
	GetFilterChanges(_ context.Context, index string) ([]any, error)
	GetFilterLogs(_ context.Context, index string) ([]*RPCLog, error)

	// Shards related
	GetShardIdList(ctx context.Context) ([]types.ShardId, error)

	// Calls related
	Call(ctx context.Context, args CallArgs, blockNrOrHash transport.BlockNumberOrHash) (hexutil.Bytes, error)
}

type BaseAPI struct {
	evmCallTimeout time.Duration
}

func NewBaseApi(evmCallTimeout time.Duration) *BaseAPI {
	return &BaseAPI{
		evmCallTimeout: evmCallTimeout,
	}
}

// APIImpl is implementation of the EthAPI interface based on remote Db access
type APIImpl struct {
	*BaseAPI

	accessor *execution.StateAccessor

	db       db.ReadOnlyDB
	msgPools []msgpool.Pool
	logs     *LogsAggregator
	logger   zerolog.Logger
}

// NewEthAPI returns APIImpl instance
func NewEthAPI(ctx context.Context, base *BaseAPI, db db.ReadOnlyDB, pools []msgpool.Pool, logger zerolog.Logger) (*APIImpl, error) {
	accessor, err := execution.NewStateAccessor()
	if err != nil {
		return nil, err
	}
	return &APIImpl{
		BaseAPI:  base,
		db:       db,
		msgPools: pools,
		logs:     NewLogsAggregator(ctx, db),
		logger:   logger,
		accessor: accessor,
	}, nil
}

func (api *APIImpl) checkShard(shardId types.ShardId) error {
	if int(shardId) >= len(api.msgPools) {
		return fmt.Errorf("shard %v doesn't exist", shardId)
	}
	return nil
}
