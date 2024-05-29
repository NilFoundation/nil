package jsonrpc

import (
	"context"
	"time"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/hexutil"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/msgpool"
	"github.com/NilFoundation/nil/rpc/filters"
	"github.com/NilFoundation/nil/rpc/transport"
	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/rs/zerolog"
)

// EthAPI is a collection of functions that are exposed in the
type EthAPI interface {
	// Block related
	GetBlockByNumber(ctx context.Context, shardId types.ShardId, number transport.BlockNumber, fullTx bool) (map[string]any, error)
	GetBlockByHash(ctx context.Context, shardId types.ShardId, hash common.Hash, fullTx bool) (map[string]any, error)
	GetBlockTransactionCountByNumber(ctx context.Context, shardId types.ShardId, number transport.BlockNumber) (*hexutil.Uint, error)
	GetBlockTransactionCountByHash(ctx context.Context, shardId types.ShardId, hash common.Hash) (*hexutil.Uint, error)

	// Message related
	GetInMessageByHash(ctx context.Context, shardId types.ShardId, hash common.Hash) (*types.Message, error)

	// Receipt related (see ./eth_receipt.go)
	GetInMessageReceipt(ctx context.Context, shardId types.ShardId, hash common.Hash) (*types.Receipt, error)

	// Account related
	GetBalance(ctx context.Context, shardId types.ShardId, address common.Address, blockNrOrHash transport.BlockNumberOrHash) (*hexutil.Big, error)
	GetTransactionCount(ctx context.Context, shardId types.ShardId, address common.Address, blockNrOrHash transport.BlockNumberOrHash) (*hexutil.Uint64, error)
	GetCode(ctx context.Context, shardId types.ShardId, address common.Address, blockNrOrHash transport.BlockNumberOrHash) (hexutil.Bytes, error)

	// Sending related
	SendRawTransaction(ctx context.Context, encoded hexutil.Bytes) (common.Hash, error)

	// Logs related
	NewFilter(_ context.Context, query filters.FilterQuery) (string, error)
	NewPendingTransactionFilter(_ context.Context) (string, error)
	NewBlockFilter(_ context.Context) (string, error)
	UninstallFilter(_ context.Context, id string) (isDeleted bool, err error)
	GetFilterChanges(_ context.Context, index string) ([]any, error)
	GetFilterLogs(_ context.Context, index string) ([]*types.Log, error)

	// Shards related
	GetShardIdList(ctx context.Context) ([]types.ShardId, error)
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

	db          db.DB
	msgPool     msgpool.Pool
	logs        *LogsAggregator
	logger      *zerolog.Logger
	blocksLRU   *lru.Cache[common.Hash, *types.Block]
	messagesLRU *lru.Cache[common.Hash, []*types.Message]
	receiptsLRU *lru.Cache[common.Hash, []*types.Receipt]
}

// NewEthAPI returns APIImpl instance
func NewEthAPI(ctx context.Context, base *BaseAPI, db db.DB, pool msgpool.Pool, logger *zerolog.Logger) *APIImpl {
	const (
		blocksLRUSize   = 128 // ~32Mb
		messagesLRUSize = 32
		receiptsLRUSize = 32
	)

	blocksLRU, err := lru.New[common.Hash, *types.Block](blocksLRUSize)
	if err != nil {
		panic(err)
	}

	messagesLRU, err := lru.New[common.Hash, []*types.Message](messagesLRUSize)
	if err != nil {
		panic(err)
	}

	receiptsLRU, err := lru.New[common.Hash, []*types.Receipt](receiptsLRUSize)
	if err != nil {
		panic(err)
	}

	return &APIImpl{
		BaseAPI:     base,
		db:          db,
		msgPool:     pool,
		logs:        NewLogsAggregator(ctx, db),
		logger:      logger,
		blocksLRU:   blocksLRU,
		messagesLRU: messagesLRU,
		receiptsLRU: receiptsLRU,
	}
}
