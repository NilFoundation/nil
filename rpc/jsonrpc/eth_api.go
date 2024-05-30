package jsonrpc

import (
	"context"
	"fmt"
	"time"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/hexutil"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/msgpool"
	"github.com/NilFoundation/nil/rpc/filters"
	"github.com/NilFoundation/nil/rpc/transport"
	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/holiman/uint256"
	"github.com/rs/zerolog"
)

type RPCInMessage struct {
	Success     bool              `json:"success"`
	BlockHash   *common.Hash      `json:"blockHash"`
	BlockNumber types.BlockNumber `json:"blockNumber"`
	From        common.Address    `json:"from"`
	GasUsed     hexutil.Uint64    `json:"gasUsed"`
	GasPrice    types.Uint256     `json:"gasPrice,omitempty"`
	GasLimit    types.Uint256     `json:"gasLimit,omitempty"`
	Hash        common.Hash       `json:"hash"`
	Seqno       hexutil.Uint64    `json:"seqno"`
	To          *common.Address   `json:"to"`
	Index       *hexutil.Uint64   `json:"index"`
	Value       types.Uint256     `json:"value"`
	ChainID     types.Uint256     `json:"chainId,omitempty"`
	Signature   common.Signature  `json:"signature"`
}

func NewRPCInMessage(message *types.Message, receipt *types.Receipt, index int, block *types.Block) *RPCInMessage {
	hash := message.Hash()
	if receipt == nil || hash != receipt.MsgHash {
		panic("Msg and receipt are not compatible")
	}

	blockHash := block.Hash()
	chainId := types.Uint256{Int: *uint256.NewInt(0)}
	gasUsed := hexutil.Uint64(receipt.GasUsed)
	msgIndex := hexutil.Uint64(index)
	seqno := hexutil.Uint64(message.Seqno)
	result := &RPCInMessage{
		Success:     receipt.Success,
		BlockHash:   &blockHash,
		BlockNumber: block.Id,
		From:        message.From,
		GasUsed:     gasUsed,
		GasPrice:    message.GasPrice,
		GasLimit:    message.GasLimit,
		Hash:        hash,
		Seqno:       seqno,
		To:          &message.To,
		Index:       &msgIndex,
		Value:       message.Value,
		ChainID:     chainId,
		Signature:   message.Signature,
	}

	return result
}

// EthAPI is a collection of functions that are exposed in the
type EthAPI interface {
	// Block related
	GetBlockByNumber(ctx context.Context, shardId types.ShardId, number transport.BlockNumber, fullTx bool) (map[string]any, error)
	GetBlockByHash(ctx context.Context, shardId types.ShardId, hash common.Hash, fullTx bool) (map[string]any, error)
	GetBlockTransactionCountByNumber(ctx context.Context, shardId types.ShardId, number transport.BlockNumber) (*hexutil.Uint, error)
	GetBlockTransactionCountByHash(ctx context.Context, shardId types.ShardId, hash common.Hash) (*hexutil.Uint, error)

	// Message related
	GetInMessageByHash(ctx context.Context, shardId types.ShardId, hash common.Hash) (*types.Message, error)

	GetInMessageByBlockHashAndIndex(ctx context.Context, hash common.Hash, index hexutil.Uint64) (*RPCInMessage, error)
	GetInMessageByBlockNumberAndIndex(ctx context.Context, number transport.BlockNumber, txIndex hexutil.Uint) (*RPCInMessage, error)
	GetRawInMessageByBlockNumberAndIndex(ctx context.Context, number transport.BlockNumber, index hexutil.Uint) (hexutil.Bytes, error)
	GetRawInMessageByBlockHashAndIndex(ctx context.Context, hash common.Hash, index hexutil.Uint) (hexutil.Bytes, error)
	GetRawInMessageByHash(ctx context.Context, hash common.Hash) (hexutil.Bytes, error)

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
	msgPools    []msgpool.Pool
	logs        *LogsAggregator
	logger      *zerolog.Logger
	blocksLRU   *lru.Cache[common.Hash, *types.Block]
	messagesLRU *lru.Cache[common.Hash, []*types.Message]
	receiptsLRU *lru.Cache[common.Hash, []*types.Receipt]
}

// NewEthAPI returns APIImpl instance
func NewEthAPI(ctx context.Context, base *BaseAPI, db db.DB, pools []msgpool.Pool, logger *zerolog.Logger) *APIImpl {
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
		msgPools:    pools,
		logs:        NewLogsAggregator(ctx, db),
		logger:      logger,
		blocksLRU:   blocksLRU,
		messagesLRU: messagesLRU,
		receiptsLRU: receiptsLRU,
	}
}

func (api *APIImpl) checkShard(shardId types.ShardId) error {
	if int(shardId) >= len(api.msgPools) {
		return fmt.Errorf("shard %v doesn't exist", shardId)
	}
	return nil
}
