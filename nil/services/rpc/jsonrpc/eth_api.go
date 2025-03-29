package jsonrpc

import (
	"context"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/filters"
	"github.com/NilFoundation/nil/nil/services/rpc/rawapi"
	"github.com/NilFoundation/nil/nil/services/rpc/transport"
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
	GetBlockByNumber(
		ctx context.Context, shardId types.ShardId, number transport.BlockNumber, fullTx bool) (*RPCBlock, error)

	/*
		@name GetBlockByHash
		@summary Returns information about a block with the given hash.
		@description Implements eth_getBlockByHash.
		@tags [Blocks]
		@param hash BlockHash
		@param fullTx FullTx
		@returns rpcBlock RPCBlock
	*/
	GetBlockByHash(ctx context.Context, hash common.Hash, fullTx bool) (*RPCBlock, error)

	/*
		@name GetBlockTransactionCountByNumber
		@summary Returns the total number of transactions recorded in the block with the given number.
		@description Implements eth_getBlockTransactionCountByNumber.
		@tags [Blocks]
		@param shardId BlockShardId
		@param number BlockNumber
		@returns transactionNumber TransactionNumber
	*/
	GetBlockTransactionCountByNumber(
		ctx context.Context, shardId types.ShardId, number transport.BlockNumber) (hexutil.Uint, error)

	/*
		@name GetBlockTransactionCountByHash
		@summary Returns the total number of transactions recorded in the block with the given hash.
		@description Implements eth_getBlockTransactionCountByHash.
		@tags [Blocks]
		@param hash BlockHash
		@returns transactionNumber TransactionNumber
	*/
	GetBlockTransactionCountByHash(ctx context.Context, hash common.Hash) (hexutil.Uint, error)

	/*
		@name GetInTransactionByHash
		@summary Returns the structure of the internal transaction with the given hash.
		@description
		@tags [Transactions]
		@param hash TransactionHash
		@returns rpcInTransaction RPCInTransaction
	*/
	GetInTransactionByHash(ctx context.Context, hash common.Hash) (*RPCInTransaction, error)

	/*
		@name GetInTransactionByBlockHashAndIndex
		@summary Returns the structure of the internal transaction with the given index
		         and contained within the block with the given hash.
		@description
		@tags [Transactions]
		@param hash BlockHash
		@param index TransactionIndex
		@returns rpcInTransaction RPCInTransaction
	*/
	GetInTransactionByBlockHashAndIndex(
		ctx context.Context, hash common.Hash, index hexutil.Uint64) (*RPCInTransaction, error)

	/*
		@name GetInTransactionByBlockNumberAndIndex
		@summary Returns the structure of the internal transaction with the given index
		         and contained within the block with the given number.
		@description
		@tags [Transactions]
		@param shardId TransactionShardId
		@param number BlockNumber
		@param index TransactionIndex
		@returns rpcInTransaction RPCInTransaction
	*/
	GetInTransactionByBlockNumberAndIndex(
		ctx context.Context,
		shardId types.ShardId,
		number transport.BlockNumber,
		index hexutil.Uint64,
	) (*RPCInTransaction, error)

	/*
		@name GetRawInTransactionByBlockNumberAndIndex
		@summary Returns the bytecode of the internal transaction with the given index
		         and contained within the block with the given number.
		@description
		@tags [Transactions]
		@param shardId TransactionShardId
		@param number BlockNumber
		@param index TransactionIndex
		@returns transactionBytecode TransactionBytecode
	*/
	GetRawInTransactionByBlockNumberAndIndex(
		ctx context.Context,
		shardId types.ShardId,
		number transport.BlockNumber,
		index hexutil.Uint64,
	) (hexutil.Bytes, error)

	/*
		@name GetRawInTransactionByBlockHashAndIndex
		@summary Returns the bytecode of the internal transaction with the given index
		         and contained within the block with the given hash.
		@description
		@tags [Transactions]
		@param hash BlockHash
		@param index TransactionIndex
		@returns transactionBytecode TransactionBytecode
	*/
	GetRawInTransactionByBlockHashAndIndex(
		ctx context.Context, hash common.Hash, index hexutil.Uint64) (hexutil.Bytes, error)

	/*
		@name GetRawInTransactionByHash
		@summary Returns the bytecode of the internal transaction with the given hash.
		@description
		@tags [Transactions]
		@param hash TransactionHash
		@returns transactionBytecode TransactionBytecode
	*/
	GetRawInTransactionByHash(ctx context.Context, hash common.Hash) (hexutil.Bytes, error)

	/*
		@name GetInTransactionReceipt
		@summary Returns the receipt for the transaction with the given hash.
		@description
		@tags [Receipts]
		@param hash TransactionHash
		@returns rpcReceipt RPCReceipt
	*/
	GetInTransactionReceipt(ctx context.Context, hash common.Hash) (*RPCReceipt, error)

	/*
		@name GetBalance
		@summary Returns the balance of the account with the given address and at the given block.
		@description Implements eth_getBalance.
		@tags [Accounts]
		@param address Address
		@param blockNumberOrHash BlockNumberOrHash
		@returns balance Balance
	*/
	GetBalance(
		ctx context.Context, address types.Address, blockNrOrHash transport.BlockNumberOrHash) (*hexutil.Big, error)

	/*
		@name GasPrice
		@summary Returns the current gas price in the network.
		@description Implements eth_gasPrice.
		@tags [Transactions]
		@param shardId GasShardId
		@returns gasPrice GasPrice
	*/
	GasPrice(ctx context.Context, shardId types.ShardId) (types.Value, error)

	/*
		@name GetTransactionCount
		@summary Returns the transaction count of the account with the given address and at the given block.
		@description Implements eth_getTransactionCount.
		@tags [Accounts]
		@param address Address
		@param blockNumberOrHash BlockNumberOrHash
		@returns transactionCount TransactionCount
	*/
	GetTransactionCount(
		ctx context.Context, address types.Address, blockNrOrHash transport.BlockNumberOrHash) (hexutil.Uint64, error)

	/*
		@name GetCode
		@summary Returns the bytecode of the contract with the given address and at the given block.
		@description Implements eth_getCode.
		@tags [Accounts]
		@param address Address
		@param blockNumberOrHash BlockNumberOrHash
		@returns contractBytecode ContractBytecode
	*/
	GetCode(
		ctx context.Context, address types.Address, blockNrOrHash transport.BlockNumberOrHash) (hexutil.Bytes, error)

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
		@name GetNumShards
		@summary Returns the number of shards in the network.
		@description
		@tags [Shards]
		@returns numShards NumShards
	*/
	GetNumShards(ctx context.Context) (uint64, error)

	/*
		@name Call
		@summary Executes a new transaction call immediately without creating a transaction.
		@description Implements eth_call.
		@tags [Calls]
		@param args CallArgs
		@param mainBlockNrOrHash BlockNumberOrHash
		@param overrides StateOverrides
		@returns callRes CallRes
	*/
	Call(
		ctx context.Context,
		args CallArgs,
		mainBlockNrOrHash transport.BlockNumberOrHash,
		overrides *StateOverrides,
	) (*CallRes, error)

	/*
		@name EstimateFee
		@summary Executes a new transaction call and returns recommended feeCredit.
		@description Implements eth_estimateGas.
		@tags [Calls]
		@param args CallArgs
		@param mainBlockNrOrHash BlockNumberOrHash
		@returns feeEstimation Value
	*/
	EstimateFee(
		ctx context.Context,
		args CallArgs,
		mainBlockNrOrHash transport.BlockNumberOrHash,
	) (*EstimateFeeRes, error)

	/*
		@name ChainId
		@summary Returns the chain ID of the current network.
		@description Implements eth_chainId.
		@tags [System]
		@returns chainId ChainId
	*/
	ChainId(ctx context.Context) (hexutil.Uint64, error)

	/*
		@name GetTokens
		@summary Returns the token balances of the account with the given address and at the given block.
		@description Implements eth_getTokens.
		@tags [Accounts]
		@param address Address
		@param blockNumberOrHash BlockNumberOrHash
		@returns balance Balance of all tokens
	*/
	GetTokens(
		ctx context.Context,
		address types.Address,
		blockNrOrHash transport.BlockNumberOrHash,
	) (map[types.TokenId]types.Value, error)
}

// EthAPI is a collection of functions that are exposed in the JSON-RPC API.
type EthAPI interface {
	EthAPIRo

	/*
		@name SendRawTransaction
		@summary Creates a new transaction or creates a contract for a previously signed transaction.
		@description Implements eth_sendRawTransaction.
		@tags [Transactions]
		@param encoded Encoded
		@returns hash TransactionHash
	*/
	SendRawTransaction(ctx context.Context, encoded hexutil.Bytes) (common.Hash, error)
}

// APIImpl is implementation of the EthAPI interface based on remote Db access
type APIImplRo struct {
	accessor *execution.StateAccessor

	logs            *LogsAggregator
	logger          logging.Logger
	clientEventsLog logging.Logger
	rawapi          rawapi.NodeApi
}

// APIImpl is implementation of the EthAPI interface based on remote Db access
type APIImpl struct {
	*APIImplRo
}

var (
	_ EthAPI   = (*APIImpl)(nil)
	_ EthAPIRo = (*APIImplRo)(nil)
)

func NewEthAPIRo(
	ctx context.Context,
	rawapi rawapi.NodeApi,
	db db.ReadOnlyDB,
	pollBlocksForLogs bool,
	logClientEvents bool,
) *APIImplRo {
	accessor := execution.NewStateAccessor()
	api := &APIImplRo{
		logger:          logging.NewLogger("eth-api"),
		accessor:        accessor,
		rawapi:          rawapi,
		clientEventsLog: logging.NewLogger("eth-api-rpc-requests"),
	}
	api.logs = NewLogsAggregator(ctx, db, pollBlocksForLogs)
	if !logClientEvents {
		api.clientEventsLog = logging.Nop()
	}
	return api
}

// NewEthAPI returns APIImpl instance
func NewEthAPI(
	ctx context.Context,
	rawapi rawapi.NodeApi,
	db db.ReadOnlyDB,
	pollBlocksForLogs bool,
	logClientEvents bool,
) *APIImpl {
	roApi := NewEthAPIRo(ctx, rawapi, db, pollBlocksForLogs, logClientEvents)
	return &APIImpl{roApi}
}

func (api *APIImplRo) Shutdown() {
	api.logs.WaitForShutdown()
}
