package client

import (
	"crypto/ecdsa"
	"fmt"
	"strings"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/NilFoundation/nil/nil/internal/contracts"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/msgpool"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
)

//go:generate go run github.com/matryer/moq -out client_generated_mock.go -rm -stub -with-resets . Client

type BatchRequest interface {
	GetBlock(shardId types.ShardId, blockId any, fullTx bool) (uint64, error)
	GetDebugBlock(shardId types.ShardId, blockId any, fullTx bool) (uint64, error)
}

// Client defines the interface for a client
// Note: This interface is designed for JSON-RPC. If you need to extend support
// for other protocols like WebSocket or gRPC in the future, you might need to
// change or extend this interface to accommodate those protocols.
type Client interface {
	RawClient
	DbClient

	CreateBatchRequest() BatchRequest
	BatchCall(BatchRequest) ([]any, error)

	EstimateFee(args *jsonrpc.CallArgs, blockId any) (types.Value, error)
	Call(args *jsonrpc.CallArgs, blockId any, stateOverride *jsonrpc.StateOverrides) (*jsonrpc.CallRes, error)
	GetCode(addr types.Address, blockId any) (types.Code, error)
	GetBlock(shardId types.ShardId, blockId any, fullTx bool) (*jsonrpc.RPCBlock, error)
	GetBlocksRange(shardId types.ShardId, from, to types.BlockNumber, fullTx bool, batchSize int) ([]*jsonrpc.RPCBlock, error)
	GetDebugBlock(shardId types.ShardId, blockId any, fullTx bool) (*jsonrpc.DebugRPCBlock, error)
	GetDebugBlocksRange(shardId types.ShardId, from, to types.BlockNumber, fullTx bool, batchSize int) ([]*jsonrpc.DebugRPCBlock, error)
	SendMessage(msg *types.ExternalMessage) (common.Hash, error)
	SendRawTransaction(data []byte) (common.Hash, error)
	GetInMessageByHash(hash common.Hash) (*jsonrpc.RPCInMessage, error)
	GetInMessageReceipt(hash common.Hash) (*jsonrpc.RPCReceipt, error)
	GetTransactionCount(address types.Address, blockId any) (types.Seqno, error)
	GetBlockTransactionCount(shardId types.ShardId, blockId any) (uint64, error)
	GetBalance(address types.Address, blockId any) (types.Value, error)
	GetShardIdList() ([]types.ShardId, error)
	GasPrice(shardId types.ShardId) (types.Value, error)
	ChainId() (types.ChainId, error)

	DeployContract(
		shardId types.ShardId, walletAddress types.Address, payload types.DeployPayload, value types.Value, pk *ecdsa.PrivateKey,
	) (common.Hash, types.Address, error)
	DeployExternal(shardId types.ShardId, deployPayload types.DeployPayload, feeCredit types.Value) (common.Hash, types.Address, error)
	SendMessageViaWallet(
		walletAddress types.Address, bytecode types.Code, feeCredit, value types.Value,
		currencies []types.CurrencyBalance, contractAddress types.Address, pk *ecdsa.PrivateKey,
	) (common.Hash, error)
	SendExternalMessage(
		bytecode types.Code, contractAddress types.Address, pk *ecdsa.PrivateKey, feeCredit types.Value,
	) (common.Hash, error)

	// GetCurrencies retrieves the contract currencies at the given address
	GetCurrencies(address types.Address, blockId any) (types.CurrenciesMap, error)

	// SetCurrencyName sets currency name
	SetCurrencyName(contractAddr types.Address, name string, pk *ecdsa.PrivateKey) (common.Hash, error)

	// ChangeCurrencyAmount mints / burns currency for the contract
	ChangeCurrencyAmount(contractAddr types.Address, amount types.Value, pk *ecdsa.PrivateKey, mint bool) (common.Hash, error)

	// GetDebugContract retrieves smart contract with its data, such as code, storage and proof
	GetDebugContract(contractAddr types.Address, blockId any) (*jsonrpc.DebugRPCContract, error)
}

func EstimateFeeExternal(c Client, msg *types.ExternalMessage, blockId any) (types.Value, error) {
	var flags types.MessageFlags
	if msg.Kind == types.DeployMessageKind {
		flags = types.NewMessageFlags(types.MessageFlagDeploy)
	}

	args := &jsonrpc.CallArgs{
		Data:  (*hexutil.Bytes)(&msg.Data),
		To:    msg.To,
		Flags: flags,
		Seqno: msg.Seqno,
	}

	return c.EstimateFee(args, blockId)
}

func SendExternalMessage(
	c Client, bytecode types.Code, contractAddress types.Address,
	pk *ecdsa.PrivateKey, feeCredit types.Value, isDeploy bool, withRetry bool,
) (common.Hash, error) {
	var kind types.MessageKind
	if isDeploy {
		kind = types.DeployMessageKind
	} else {
		kind = types.ExecutionMessageKind
	}

	// Get the sequence number for the wallet
	seqno, err := c.GetTransactionCount(contractAddress, "pending")
	if err != nil {
		return common.EmptyHash, err
	}

	// Create the message with the bytecode to run
	extMsg := &types.ExternalMessage{
		To:        contractAddress,
		Data:      bytecode,
		Seqno:     seqno,
		Kind:      kind,
		FeeCredit: feeCredit,
	}

	if feeCredit.IsZero() {
		var err error
		if feeCredit, err = EstimateFeeExternal(c, extMsg, "latest"); err != nil {
			return common.EmptyHash, err
		}
	}
	extMsg.FeeCredit = feeCredit

	if withRetry {
		return sendExternalMessageWithSeqnoRetry(c, extMsg, pk)
	}

	if pk != nil {
		err = extMsg.Sign(pk)
		if err != nil {
			return common.EmptyHash, err
		}
	}

	return c.SendMessage(extMsg)
}

// sendExternalMessageWithSeqnoRetry tries to send an external message increasing seqno if needed.
// Can be used to ensure sending messages to common contracts like Faucet.
func sendExternalMessageWithSeqnoRetry(c Client, msg *types.ExternalMessage, pk *ecdsa.PrivateKey) (common.Hash, error) {
	var err error
	for range 20 {
		if pk != nil {
			if err := msg.Sign(pk); err != nil {
				return common.EmptyHash, err
			}
		}

		var txHash common.Hash
		txHash, err = c.SendMessage(msg)
		if err == nil {
			return txHash, nil
		}
		if !strings.Contains(err.Error(), msgpool.NotReplaced.String()) &&
			!strings.Contains(err.Error(), msgpool.SeqnoTooLow.String()) {
			return common.EmptyHash, err
		}

		msg.Seqno++
	}
	return common.EmptyHash, fmt.Errorf("failed to send message in 20 retries, getting %w", err)
}

func SendMessageViaWallet(
	c Client, walletAddress types.Address, bytecode types.Code, feeCredit, value types.Value,
	currencies []types.CurrencyBalance, contractAddress types.Address, pk *ecdsa.PrivateKey, isDeploy bool,
) (common.Hash, error) {
	var kind types.MessageKind
	if isDeploy {
		kind = types.DeployMessageKind
	} else {
		kind = types.ExecutionMessageKind
	}

	intMsg := &types.InternalMessagePayload{
		Data:        bytecode,
		To:          contractAddress,
		Value:       value,
		ForwardKind: types.ForwardKindRemaining,
		Currency:    currencies,
		Kind:        kind,
	}

	intMsgData, err := intMsg.MarshalSSZ()
	if err != nil {
		return common.EmptyHash, err
	}

	calldataExt, err := contracts.NewCallData(contracts.NameWallet, "send", intMsgData)
	if err != nil {
		return common.EmptyHash, err
	}

	return c.SendExternalMessage(calldataExt, walletAddress, pk, feeCredit)
}
