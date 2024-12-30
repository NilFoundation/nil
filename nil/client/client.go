package client

import (
	"context"
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
	BatchCall(ctx context.Context, req BatchRequest) ([]any, error)

	EstimateFee(ctx context.Context, args *jsonrpc.CallArgs, blockId any) (types.Value, error)
	Call(ctx context.Context, args *jsonrpc.CallArgs, blockId any, stateOverride *jsonrpc.StateOverrides) (*jsonrpc.CallRes, error)
	GetCode(ctx context.Context, addr types.Address, blockId any) (types.Code, error)
	GetBlock(ctx context.Context, shardId types.ShardId, blockId any, fullTx bool) (*jsonrpc.RPCBlock, error)
	GetBlocksRange(ctx context.Context, shardId types.ShardId, from, to types.BlockNumber, fullTx bool, batchSize int) ([]*jsonrpc.RPCBlock, error)
	GetDebugBlock(ctx context.Context, shardId types.ShardId, blockId any, fullTx bool) (*jsonrpc.DebugRPCBlock, error)
	GetDebugBlocksRange(ctx context.Context, shardId types.ShardId, from, to types.BlockNumber, fullTx bool, batchSize int) ([]*jsonrpc.DebugRPCBlock, error)
	SendMessage(ctx context.Context, msg *types.ExternalMessage) (common.Hash, error)
	SendRawTransaction(ctx context.Context, data []byte) (common.Hash, error)
	GetInMessageByHash(ctx context.Context, hash common.Hash) (*jsonrpc.RPCInMessage, error)
	GetInMessageReceipt(ctx context.Context, hash common.Hash) (*jsonrpc.RPCReceipt, error)
	GetTransactionCount(ctx context.Context, address types.Address, blockId any) (types.Seqno, error)
	GetBlockTransactionCount(ctx context.Context, shardId types.ShardId, blockId any) (uint64, error)
	GetBalance(ctx context.Context, address types.Address, blockId any) (types.Value, error)
	GetShardIdList(ctx context.Context) ([]types.ShardId, error)
	GasPrice(ctx context.Context, shardId types.ShardId) (types.Value, error)
	ChainId(ctx context.Context) (types.ChainId, error)

	DeployContract(
		ctx context.Context, shardId types.ShardId, walletAddress types.Address, payload types.DeployPayload, value types.Value, pk *ecdsa.PrivateKey,
	) (common.Hash, types.Address, error)
	DeployExternal(ctx context.Context, shardId types.ShardId, deployPayload types.DeployPayload, feeCredit types.Value) (common.Hash, types.Address, error)
	SendMessageViaWallet(
		ctx context.Context, walletAddress types.Address, bytecode types.Code, feeCredit, value types.Value,
		currencies []types.CurrencyBalance, contractAddress types.Address, pk *ecdsa.PrivateKey,
	) (common.Hash, error)
	SendExternalMessage(
		ctx context.Context, bytecode types.Code, contractAddress types.Address, pk *ecdsa.PrivateKey, feeCredit types.Value,
	) (common.Hash, error)

	// GetCurrencies retrieves the contract currencies at the given address
	GetCurrencies(ctx context.Context, address types.Address, blockId any) (types.CurrenciesMap, error)

	// SetCurrencyName sets currency name
	SetCurrencyName(ctx context.Context, contractAddr types.Address, name string, pk *ecdsa.PrivateKey) (common.Hash, error)

	// ChangeCurrencyAmount mints / burns currency for the contract
	ChangeCurrencyAmount(ctx context.Context, contractAddr types.Address, amount types.Value, pk *ecdsa.PrivateKey, mint bool) (common.Hash, error)

	// GetDebugContract retrieves smart contract with its data, such as code, storage and proof
	GetDebugContract(ctx context.Context, contractAddr types.Address, blockId any) (*jsonrpc.DebugRPCContract, error)
}

func EstimateFeeExternal(ctx context.Context, c Client, msg *types.ExternalMessage, blockId any) (types.Value, error) {
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

	return c.EstimateFee(ctx, args, blockId)
}

func SendExternalMessage(
	ctx context.Context, c Client, bytecode types.Code, contractAddress types.Address,
	pk *ecdsa.PrivateKey, feeCredit types.Value, isDeploy bool, withRetry bool,
) (common.Hash, error) {
	var kind types.MessageKind
	if isDeploy {
		kind = types.DeployMessageKind
	} else {
		kind = types.ExecutionMessageKind
	}

	// Get the sequence number for the wallet
	seqno, err := c.GetTransactionCount(ctx, contractAddress, "pending")
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
		if feeCredit, err = EstimateFeeExternal(ctx, c, extMsg, "latest"); err != nil {
			return common.EmptyHash, err
		}
	}
	extMsg.FeeCredit = feeCredit

	if withRetry {
		return sendExternalMessageWithSeqnoRetry(ctx, c, extMsg, pk)
	}

	if pk != nil {
		err = extMsg.Sign(pk)
		if err != nil {
			return common.EmptyHash, err
		}
	}

	return c.SendMessage(ctx, extMsg)
}

// sendExternalMessageWithSeqnoRetry tries to send an external message increasing seqno if needed.
// Can be used to ensure sending messages to common contracts like Faucet.
func sendExternalMessageWithSeqnoRetry(ctx context.Context, c Client, msg *types.ExternalMessage, pk *ecdsa.PrivateKey) (common.Hash, error) {
	var err error
	for range 20 {
		if pk != nil {
			if err := msg.Sign(pk); err != nil {
				return common.EmptyHash, err
			}
		}

		var txHash common.Hash
		txHash, err = c.SendMessage(ctx, msg)
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
	ctx context.Context, c Client, walletAddress types.Address, bytecode types.Code, feeCredit, value types.Value,
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

	return c.SendExternalMessage(ctx, calldataExt, walletAddress, pk, feeCredit)
}
