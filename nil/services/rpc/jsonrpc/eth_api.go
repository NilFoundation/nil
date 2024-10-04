package jsonrpc

import (
	"context"
	"fmt"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/msgpool"
	"github.com/NilFoundation/nil/nil/services/rpc/filters"
	"github.com/NilFoundation/nil/nil/services/rpc/rawapi"
	"github.com/NilFoundation/nil/nil/services/rpc/transport"
	"github.com/rs/zerolog"
)

type EthAPIRo interface {
	/*
		@name GetBlockByNumber
		@summary Returns information about a block with the given number.
		@description Implements eth_getBlockByNumber.
		@tags [Blocks]
		@param shardId BlockShardId
		@param blockNumber BlockNumber
		@param fullTx FullTx
		@returns rpcBlock RPCBlock
	*/
	GetBlockByNumber(ctx context.Context, shardId types.ShardId, number transport.BlockNumber, fullTx bool) (*RPCBlock, error)

	/*
		@name GetBlockByHash
		@summary Returns information about a block with the given hash.
		@description Implements eth_getBlockByHash.
		@tags [Blocks]
		@param shardId BlockShardId
		@param hash BlockHash
		@param fullTx FullTx
		@returns rpcBlock RPCBlock
	*/
	GetBlockByHash(ctx context.Context, shardId types.ShardId, hash common.Hash, fullTx bool) (*RPCBlock, error)

	/*
		@name GetBlockTransactionCountByNumber
		@summary Returns the total number of transactions recorded in the block with the given number.
		@description Implements eth_getBlockTransactionCountByNumber.
		@tags [Blocks]
		@param shardId BlockShardId
		@param number BlockNumber
		@returns transactionNumber TransactionNumber
	*/
	GetBlockTransactionCountByNumber(ctx context.Context, shardId types.ShardId, number transport.BlockNumber) (hexutil.Uint, error)

	/*
		@name GetBlockTransactionCountByHash
		@summary Returns the total number of transactions recorded in the block with the given hash.
		@description Implements eth_getBlockTransactionCountByHash.
		@tags [Blocks]
		@param shardId BlockShardId
		@param hash BlockHash
		@returns transactionNumber TransactionNumber
	*/
	GetBlockTransactionCountByHash(ctx context.Context, shardId types.ShardId, hash common.Hash) (hexutil.Uint, error)

	/*
		@name GetInMessageByHash
		@summary Returns the structure of the internal message with the given hash.
		@description
		@tags [Messages]
		@param shardId MessageShardId
		@param hash MessageHash
		@returns rpcInMessage RPCInMessage
	*/
	GetInMessageByHash(ctx context.Context, shardId types.ShardId, hash common.Hash) (*RPCInMessage, error)

	/*
		@name GetInMessageByBlockHashAndIndex
		@summary Returns the structure of the internal message with the given index and contained within the block with the given hash.
		@description
		@tags [Messages]
		@param shardId MessageShardId
		@param hash BlockHash
		@param index MessageIndex
		@returns rpcInMessage RPCInMessage
	*/
	GetInMessageByBlockHashAndIndex(ctx context.Context, shardId types.ShardId, hash common.Hash, index hexutil.Uint64) (*RPCInMessage, error)

	/*
		@name GetInMessageByBlockNumberAndIndex
		@summary Returns the structure of the internal message with the given index and contained within the block with the given number.
		@description
		@tags [Messages]
		@param shardId MessageShardId
		@param number BlockNumber
		@param index MessageIndex
		@returns rpcInMessage RPCInMessage
	*/
	GetInMessageByBlockNumberAndIndex(ctx context.Context, shardId types.ShardId, number transport.BlockNumber, index hexutil.Uint64) (*RPCInMessage, error)

	/*
		@name GetRawInMessageByBlockNumberAndIndex
		@summary Returns the bytecode of the internal message with the given index and contained within the block with the given number.
		@description
		@tags [Messages]
		@param shardId MessageShardId
		@param number BlockNumber
		@param index MessageIndex
		@returns messageBytecode MessageBytecode
	*/
	GetRawInMessageByBlockNumberAndIndex(ctx context.Context, shardId types.ShardId, number transport.BlockNumber, index hexutil.Uint64) (hexutil.Bytes, error)

	/*
		@name GetRawInMessageByBlockHashAndIndex
		@summary Returns the bytecode of the internal message with the given index and contained within the block with the given hash.
		@description
		@tags [Messages]
		@param shardId MessageShardId
		@param hash BlockHash
		@param index MessageIndex
		@returns messageBytecode MessageBytecode
	*/
	GetRawInMessageByBlockHashAndIndex(ctx context.Context, shardId types.ShardId, hash common.Hash, index hexutil.Uint64) (hexutil.Bytes, error)

	/*
		@name GetRawInMessageByHash
		@summary Returns the bytecode of the internal message with the given hash.
		@description
		@tags [Messages]
		@param shardId MessageShardId
		@param hash MessageHash
		@returns messageBytecode MessageBytecode
	*/
	GetRawInMessageByHash(ctx context.Context, shardId types.ShardId, hash common.Hash) (hexutil.Bytes, error)

	/*
		@name GetInMessageReceipt
		@summary Returns the receipt for the message with the given hash.
		@description
		@tags [Receipts]
		@param shardId MessageShardId
		@param hash MessageHash
		@returns rpcReceipt RPCReceipt
	*/
	GetInMessageReceipt(ctx context.Context, shardId types.ShardId, hash common.Hash) (*RPCReceipt, error)

	/*
		@name GetBalance
		@summary Returns the balance of the account with the given address and at the given block.
		@description Implements eth_getBalance.
		@tags [Accounts]
		@param address Address
		@param blockNumberOrHash BlockNumberOrHash
		@returns balance Balance
	*/
	GetBalance(ctx context.Context, address types.Address, blockNrOrHash transport.BlockNumberOrHash) (*hexutil.Big, error)

	/*
		@name GasPrice
		@summary Returns the current gas price in the network.
		@description Implements eth_gasPrice.
		@tags [Transactions]
		@param shardId GasShardId
		@returns gasPrice GasPrice
	*/
	GasPrice(ctx context.Context, shardId types.ShardId) (*hexutil.Big, error)

	/*
		@name GetTransactionCount
		@summary Returns the transaction count of the account with the given address and at the given block.
		@description Implements eth_getTransactionCount.
		@tags [Accounts]
		@param address Address
		@param blockNumberOrHash BlockNumberOrHash
		@returns transactionCount TransactionCount
	*/
	GetTransactionCount(ctx context.Context, address types.Address, blockNrOrHash transport.BlockNumberOrHash) (*hexutil.Uint64, error)

	/*
		@name GetCode
		@summary Returns the bytecode of the contract with the given address and at the given block.
		@description Implements eth_getCode.
		@tags [Accounts]
		@param address Address
		@param blockNumberOrHash BlockNumberOrHash
		@returns contractBytecode ContractBytecode
	*/
	GetCode(ctx context.Context, address types.Address, blockNrOrHash transport.BlockNumberOrHash) (hexutil.Bytes, error)

	/*
		@name NewFilter
		@summary Creates a new filter.
		@description
		@tags [Filters]
		@param query FilterQuery
		@returns filterId FilterId
	*/
	NewFilter(_ context.Context, query filters.FilterQuery) (string, error)

	/*
		@name NewPendingTransactionFilter
		@summary Creates a new pending transactions filter.
		@description Implements eth_newPendingTransactionFilter.
		@tags [Filters]
		@returns filterId FilterId
	*/
	NewPendingTransactionFilter(_ context.Context) (string, error)

	/*
		@name NewBlockFilter
		@summary Creates a new block filter.
		@description Implements eth_newBlockFilter.
		@tags [Filters]
		@returns filterId FilterId
	*/
	NewBlockFilter(_ context.Context) (string, error)

	/*
		@name UninstallFilter
		@summary Uninstalls the filter with the given id.
		@description Implements eth_uninstallFilter.
		@param id UninstallFilterId
		@tags [Filters]
		@returns isDeleted IsDeleted
	*/
	UninstallFilter(_ context.Context, id string) (isDeleted bool, err error)

	/*
		@name GetFilterChanges
		@summary Polls the filter with the given id for all changes.
		@description Implements eth_getFilterChanges.
		@tags [Filters]
		@param id PollFilterId
		@returns filterChanges FilterChanges
	*/
	GetFilterChanges(_ context.Context, id string) ([]any, error)

	/*
		@name GetFilterLogs
		@summary Polls the filter with the given id for logs.
		@description Implements eth_getFilterLogs.
		@tags [Filters]
		@param id PollFilterId
		@returns filterLogs FilterLogs
	*/
	GetFilterLogs(_ context.Context, id string) ([]*RPCLog, error)

	/*
		@name GetShardsIdList
		@summary Retrieves a list of IDs of all shards.
		@description
		@tags [Shards]
		@returns shardIds ShardIds
	*/
	GetShardIdList(ctx context.Context) ([]types.ShardId, error)

	/*
		@name Call
		@summary Executes a new message call immediately without creating a transaction.
		@description Implements eth_call.
		@tags [Calls]
		@param args CallArgs
		@param mainBlockNrOrHash BlockNumberOrHash
		@param overrides StateOverrides
		@returns callRes CallRes
	*/
	Call(ctx context.Context, args CallArgs, mainBlockNrOrHash transport.BlockNumberOrHash, overrides *StateOverrides) (*CallRes, error)

	/*
		@name EstimateFee
		@summary Executes a new message call and returns recommended feeCredit.
		@description Implements eth_estimateGas.
		@tags [Calls]
		@param args CallArgs
		@param mainBlockNrOrHash BlockNumberOrHash
		@returns feeEstimation Value
	*/
	EstimateFee(ctx context.Context, args CallArgs, mainBlockNrOrHash transport.BlockNumberOrHash) (types.Value, error)

	/*
		@name ChainId
		@summary Returns the chain ID of the current network.
		@description Implements eth_chainId.
		@tags [System]
		@returns chainId ChainId
	*/
	ChainId(ctx context.Context) (hexutil.Uint64, error)

	/*
		@name GetCurrencies
		@summary Returns the currency balances of the account with the given address and at the given block.
		@description Implements eth_getCurrencies.
		@tags [Accounts]
		@param address Address
		@param blockNumberOrHash BlockNumberOrHash
		@returns balance Balance of all currencies
	*/
	GetCurrencies(ctx context.Context, address types.Address, blockNrOrHash transport.BlockNumberOrHash) (map[string]*hexutil.Big, error)
}

// EthAPI is a collection of functions that are exposed in the JSON-RPC API.
type EthAPI interface {
	EthAPIRo

	/*
		@name SendRawTransaction
		@summary Creates a new message or creates a contract for a previously signed message.
		@description Implements eth_sendRawTransaction.
		@tags [Transactions]
		@param encoded Encoded
		@returns hash MessageHash
	*/
	SendRawTransaction(ctx context.Context, encoded hexutil.Bytes) (common.Hash, error)
}

// APIImpl is implementation of the EthAPI interface based on remote Db access
type APIImpl struct {
	accessor *execution.StateAccessor

	db       db.ReadOnlyDB
	msgPools []msgpool.Pool
	logs     *LogsAggregator
	logger   zerolog.Logger
	rawapi   rawapi.NodeApi
}

var (
	_ EthAPI   = (*APIImpl)(nil)
	_ EthAPIRo = (*APIImpl)(nil)
)

// NewEthAPI returns APIImpl instance
func NewEthAPI(ctx context.Context, rawapi rawapi.NodeApi, db db.ReadOnlyDB, pools []msgpool.Pool, pollBlocksForLogs bool) (*APIImpl, error) {
	accessor, err := execution.NewStateAccessor()
	if err != nil {
		return nil, err
	}
	api := &APIImpl{
		db:       db,
		msgPools: pools,
		logger:   logging.NewLogger("eth-api"),
		accessor: accessor,
		rawapi:   rawapi,
	}
	api.logs = NewLogsAggregator(ctx, db, pollBlocksForLogs)
	return api, nil
}

func (api *APIImpl) checkShard(shardId types.ShardId) error {
	if int(shardId) >= len(api.msgPools) {
		return fmt.Errorf("%w (%d)", ErrShardNotFound, shardId)
	}
	return nil
}

func (api *APIImpl) Shutdown() {
	api.logs.WaitForShutdown()
}
