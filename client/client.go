package client

import (
	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/rpc/jsonrpc"
	"github.com/NilFoundation/nil/rpc/transport"
)

// Client defines the interface for a client
// Note: This interface is designed for JSON-RPC. If you need to extend support
// for other protocols like WebSocket or gRPC in the future, you might need to
// change or extend this interface to accommodate those protocols.
type Client interface {
	// Call sends a request to the server with the given method and parameters,
	// and returns the response as json.RawMessage, or an error if the call fails
	Call(method string, params ...any) (map[string]any, error)

	GetCode(addr types.Address, blockNrOrHash transport.BlockNumberOrHash) (types.Code, error)
	GetBlockByHash(shardId types.ShardId, hash common.Hash, fullTx bool) (*jsonrpc.RPCBlock, error)
	GetBlockByNumber(shardId types.ShardId, num transport.BlockNumber, fullTx bool) (*jsonrpc.RPCBlock, error)
	SendMessage(msg *types.Message) (common.Hash, error)
	SendRawTransaction(data []byte) (common.Hash, error)
	GetInMessageByHash(shardId types.ShardId, hash common.Hash) (*jsonrpc.RPCInMessage, error)
	GetInMessageReceipt(shardId types.ShardId, hash common.Hash) (*jsonrpc.RPCReceipt, error)
	GetTransactionCount(address types.Address, blockNrOrHash transport.BlockNumberOrHash) (types.Seqno, error)
	GetBlockTransactionCountByNumber(shardId types.ShardId, number transport.BlockNumber) (uint64, error)
	GetBlockTransactionCountByHash(shardId types.ShardId, hash common.Hash) (uint64, error)
}
