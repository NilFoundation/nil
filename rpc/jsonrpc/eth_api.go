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
	GetBlockTransactionCountByNumber(ctx context.Context, shardId types.ShardId, number transport.BlockNumber) (*hexutil.Uint, error)
	
	/*
	@name GetBlockTransactionCountByHash
	@summary Returns the total number of transactions recorded in the block with the given hash.
	@description Implements eth_getBlockTransactionCountByHash.
	@tags [Blocks]
	@param shardId BlockShardId
	@param hash BlockHash
	@returns transactionNumber TransactionNumber
	*/
	GetBlockTransactionCountByHash(ctx context.Context, shardId types.ShardId, hash common.Hash) (*hexutil.Uint, error)

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
	@name SendRawTransaction
	@summary Creates a new message or creates a contract for a previously signed message.
	@description Implements eth_sendRawTransaction.
	@tags [Transactions]
	@param encoded Encoded
	@returns hash MessageHash
	*/
	SendRawTransaction(ctx context.Context, encoded hexutil.Bytes) (common.Hash, error)

	/*
	@name NewFilter
	@summary Creates a new filter.
	@description
	@tags [Filters]
	@param query FilterQuery
	@returns FilterId filterId
	*/
	NewFilter(_ context.Context, query filters.FilterQuery) (string, error)

	/*
	@name NewPendingTransactionFilter
	@summary Creates a new pending transactions filter.
	@description Implements eth_newPendingTransactionFilter.
	@tags [Filters]
	@returns FilterId filterId
	*/
	NewPendingTransactionFilter(_ context.Context) (string, error)

	/*
	@name NewBlockFilter
	@summary Creates a new block filter.
	@description Implements eth_newBlockFilter.
	@tags [Filters]
	@returns FilterId filterId
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
	@summary Retrieves a list of ids of all shards.
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
	@param blockNrOrHash BlockNumberOrHash
	@returns returnedValue ReturnedValue
	*/
	Call(ctx context.Context, args CallArgs, blockNrOrHash transport.BlockNumberOrHash) (hexutil.Bytes, error)

	//@component Address address string "The address of the account or contract."
	//@component TransactionCount transactionCount integer "The transaction count of the account."
	//@component ContractBytecode contractBytecode string "The bytecode of the contract."

	//@component BlockNumberOrHash blockNumberOrHash object "The number/hash of the block."
	//@componentprop RequireCanonical requireCanonical boolean true "The flag that determines whether the block must be a part of the canonical chain."
	//@componentprop BlockHash blockHash string false "(Optional) The hash of the block. Either this or BlockNumber is required."
	//@componentprop BlockNumber blockNumber integer false "(Optional) The number of the block. Either this or BlockHash is required."

	//@component Balance balance integer "The balance of the account."
	//@component BlockShardId shardId integer "The ID of the shard where the block was generated."
	//@component MessageShardId shardId integer "The ID of the shard where the message was recorded."

	//@component RPCBlock rpcBlock object "The block whose information was requested."
	//@componentprop Hash hash string true "The hash of the block."
	//@componentprop Messages messages array true "The messages included in the block."
	//@componentprop Number number integer true "The block number."
	//@componentprop ParentHash parentHash string true "The hash of the parent block."
	//@componentprop ReceiptsRoot receiptsRoot string true "The root of the block receipts."
	//@componentprop ShardId shardId integer true "The ID of the shard where the block was generated."

	//@component BlockHash hash string "The hash of the block whose information is requested."
	//@component MessageHash hash string "The hash of the message."
	//@component BlockNumber blockNumber integer "The number of the block whose information is requested."
	//@component TransactionNumber transactionNumber integer "The number of transactions contained within the block."

	//@component RPCInMessage rpcInMessage object "The message whose information is requested."
	//@componentprop BlockHash blockHash string true "The hash of the block containing the message."
	//@componentprop BlockNumber blockNumber integer true "The number of the block containin the message."
	//@componentprop ChainId chainId integer true "The number of the chain containing the message."
	//@componentprop From from string true "The address from where the message was sent."
	//@componentprop GasLimit gasLimit string true "The gas limit for the message."
	//@componentprop GasPrice gasPrice string true "The gas price paid for the message."
	//@componentprop GasUsed gasUsed string true "The amount of gas spent on the message."
	//@componentprop Hash hash string true "The message hash."
	//@componentprop Index index string true "The message index."
	//@componentprop Seqno seqno string true "The sequence number of the message."
	//@componentprop Signature signature string true "The message signature."
	//@componentprop Success success boolean true "The flag that shows whether the message was successful."
	//@componentprop To to string true "The address where the message was sent."
	//@componentprop Value value string true "The message value."

	//@component MessageIndex index integer "The index of the message whose information is requested."
	//@component MessageBytecode code string "The bytecode of the requested message."

	//@component RPCReceipt rpcReceipt object "The receipt whose structure is requested."
	//@componentprop BlockHash blockHash string true "The hash of the block containing the message whose receipt is requested."
	//@componentprop BlockNumber blockNumber integer true "The number of the block containin the message whose receipt is requested."
	//@componentprop Bloom bloom string true "The receipt bloom filter."
	//@componentprop ContractAddress contractAddress string true "The address of the contract that has originated the message whose receipt is requested."
	//@componentprop GasUsed gasUsed string true "The amount of gas spent on the message whose receipt is requested."
	//@componentprop Logs logs array true "The logs attached to the receipt."
	//@componentprop MessageHash messageHash string true "The hash of the message whose receipt is requested."
	//@componentprop MessageIndex messageIndex integer true "The index of the message whose receipt is requested."
	//@componentprop OutMsgIndex outMsgIndex integer true "The index of the outgoing message whose receipt is requested."
	//@componentprop Success success boolean true "The flag that shows whether the message was successful."

	//@component Encoded encoded string "The encoded bytecode of the message."

	//@component FilterQuery filterQuery object "The query structure of the filter."
	//@componentprop BlockHash blockHash string false "The hash of the blocks whose logs should be retrieved by the filter."
	//@componentprop FromBlock fromBlock integer false "The beginning of the range of the blocks whose logs should be retrieved by the filter."
	//@componentprop ToBlock toBlock integer false "The end of the range of the blocks whose logs should be retrieved by the filter."
	//@componentprop Addresses addresses array true "The addresses of the accounts/contracts the logs for whose events should be retrieved by the filter."
	//@componentprop Topics topics array true "The topics of the events whose lgos should be retrieved by the filter."

    //@component CallArgs callArgs object "The arguments for the message call."
    //@componentprop From from string true "The address from which the message must be called."
    //@componentprop GasLimit gasLimit string true "The gas limit for the message."
	//@componentprop GasPrice gasPrice string false "The gas price for the message."
    //@componentprop Value value integer true "The message value."
    //@componentprop Seqno seqno integer true "The sequence number of the message."
    //@componentprop Data data string true "The encoded bytecode of the message."
    //@componentprop Input input string false "The message input."
    //@component propr ChainId chainId integer "The chain id."

	//@component IsDeleted isDeleted boolean "The flag that shows whether the filter has been successfully deleted."
	//@component PollFilterId id string "The id of the filter that should be polled."
	//@component UninstallFilterId id string "The id of the filter that should be uninstalled."
	//@component FilterId id string "The id of the filter."
	//@component FilterChanges filterChanges array "The array of logs, block headers or pending transactions that have occurred since the last poll of the filter." 
	//@component FilterLogs filterLogs array "The array of logs that have been recorded since the last poll of the filter."
	//@component ShardIds shardIds array "The array of shard ids."
	//@component ReturnedValue returnedValue string "The returned value of the executed contract."
    //@component FullTx fullTx boolean "The flag that determines whether full transaction information is returned in the output."
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
