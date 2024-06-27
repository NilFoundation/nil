package client

import (
	"crypto/ecdsa"
	"encoding/json"
	"math/big"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/rpc/jsonrpc"
)

// Client defines the interface for a client
// Note: This interface is designed for JSON-RPC. If you need to extend support
// for other protocols like WebSocket or gRPC in the future, you might need to
// change or extend this interface to accommodate those protocols.
type Client interface {
	// RawCall sends a request to the server with the given method and parameters,
	// and returns the response as json.RawMessage, or an error if the call fails
	RawCall(method string, params ...any) (json.RawMessage, error)

	Call(args *jsonrpc.CallArgs) (string, error)
	GetCode(addr types.Address, blockId any) (types.Code, error)
	GetBlock(shardId types.ShardId, blockId any, fullTx bool) (*jsonrpc.RPCBlock, error)
	SendMessage(msg *types.ExternalMessage) (common.Hash, error)
	SendRawTransaction(data []byte) (common.Hash, error)
	GetInMessageByHash(shardId types.ShardId, hash common.Hash) (*jsonrpc.RPCInMessage, error)
	GetInMessageReceipt(shardId types.ShardId, hash common.Hash) (*jsonrpc.RPCReceipt, error)
	GetTransactionCount(address types.Address, blockId any) (types.Seqno, error)
	GetBlockTransactionCount(shardId types.ShardId, blockId any) (uint64, error)
	GetBalance(address types.Address, blockId any) (*types.Uint256, error)
	GetShardIdList() ([]types.ShardId, error)
	GasPrice(shardId types.ShardId) (*types.Uint256, error)

	DeployContract(
		shardId types.ShardId, walletAddress types.Address, payload types.DeployPayload, value *types.Uint256, pk *ecdsa.PrivateKey,
	) (common.Hash, types.Address, error)
	DeployExternal(shardId types.ShardId, deployPayload types.DeployPayload) (common.Hash, types.Address, error)
	SendMessageViaWallet(
		walletAddress types.Address, bytecode types.Code, gasLimit *types.Uint256, value *types.Uint256,
		currencies []types.CurrencyBalance, contractAddress types.Address, pk *ecdsa.PrivateKey,
	) (common.Hash, error)
	SendExternalMessage(
		bytecode types.Code, contractAddress types.Address, pk *ecdsa.PrivateKey,
	) (common.Hash, error)

	TopUpViaFaucet(contractAddress types.Address, amount *types.Uint256) (common.Hash, error)

	// GetCurrencies retrieves the contract currencies at the given address
	GetCurrencies(address types.Address, blockId any) (types.CurrenciesMap, error)

	// CurrencyMint creates currency for the contract
	CurrencyCreate(contractAddr types.Address, amount *big.Int, name string, withdraw bool, pk *ecdsa.PrivateKey) (common.Hash, error)

	// CurrencyWithdraw transfers currency to the contract
	CurrencyWithdraw(contractAddr types.Address, amount *big.Int, toAddr types.Address, pk *ecdsa.PrivateKey) (common.Hash, error)

	// CurrencyMint mints currency for the contract
	CurrencyMint(contractAddr types.Address, amount *big.Int, withdraw bool, pk *ecdsa.PrivateKey) (common.Hash, error)
}
