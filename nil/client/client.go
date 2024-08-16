package client

import (
	"crypto/ecdsa"
	"encoding/json"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
)

type BatchRequest interface {
	GetBlock(shardId types.ShardId, blockId any, fullTx bool) (uint64, error)
	GetDebugBlock(shardId types.ShardId, blockId any, fullTx bool) (uint64, error)
}

// Client defines the interface for a client
// Note: This interface is designed for JSON-RPC. If you need to extend support
// for other protocols like WebSocket or gRPC in the future, you might need to
// change or extend this interface to accommodate those protocols.
type Client interface {
	DbClient

	// RawCall sends a request to the server with the given method and parameters,
	// and returns the response as json.RawMessage, or an error if the call fails
	RawCall(method string, params ...any) (json.RawMessage, error)

	// PlainTextCall sends request as is and returns raw output.
	// Function is useful mainly for testing purposes.
	PlainTextCall(requestBody []byte) (json.RawMessage, error)

	CreateBatchRequest() BatchRequest
	BatchCall(BatchRequest) ([]any, error)

	Call(args *jsonrpc.CallArgs, blockId any, stateOverride *jsonrpc.StateOverrides) (*jsonrpc.CallRes, error)
	GetCode(addr types.Address, blockId any) (types.Code, error)
	GetBlock(shardId types.ShardId, blockId any, fullTx bool) (*jsonrpc.RPCBlock, error)
	GetDebugBlock(shardId types.ShardId, blockId any, fullTx bool) (*jsonrpc.HexedDebugRPCBlock, error)
	SendMessage(msg *types.ExternalMessage) (common.Hash, error)
	SendRawTransaction(data []byte) (common.Hash, error)
	GetInMessageByHash(shardId types.ShardId, hash common.Hash) (*jsonrpc.RPCInMessage, error)
	GetInMessageReceipt(shardId types.ShardId, hash common.Hash) (*jsonrpc.RPCReceipt, error)
	GetTransactionCount(address types.Address, blockId any) (types.Seqno, error)
	GetBlockTransactionCount(shardId types.ShardId, blockId any) (uint64, error)
	GetBalance(address types.Address, blockId any) (types.Value, error)
	GetShardIdList() ([]types.ShardId, error)
	GasPrice(shardId types.ShardId) (types.Value, error)
	ChainId() (types.ChainId, error)

	DeployContract(
		shardId types.ShardId, walletAddress types.Address, payload types.DeployPayload, value types.Value, pk *ecdsa.PrivateKey,
	) (common.Hash, types.Address, error)
	DeployExternal(shardId types.ShardId, deployPayload types.DeployPayload) (common.Hash, types.Address, error)
	SendMessageViaWallet(
		walletAddress types.Address, bytecode types.Code, feeCredit types.Value, value types.Value,
		currencies []types.CurrencyBalance, contractAddress types.Address, pk *ecdsa.PrivateKey,
	) (common.Hash, error)
	SendExternalMessage(
		bytecode types.Code, contractAddress types.Address, pk *ecdsa.PrivateKey,
	) (common.Hash, error)

	TopUpViaFaucet(contractAddress types.Address, amount types.Value) (common.Hash, error)

	// GetCurrencies retrieves the contract currencies at the given address
	GetCurrencies(address types.Address, blockId any) (types.CurrenciesMap, error)

	// SetCurrencyName sets currency name
	SetCurrencyName(contractAddr types.Address, name string, pk *ecdsa.PrivateKey) (common.Hash, error)

	// CurrencyMint mints currency for the contract
	CurrencyMint(contractAddr types.Address, amount types.Value, pk *ecdsa.PrivateKey) (common.Hash, error)
}
