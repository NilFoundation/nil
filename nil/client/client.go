package client

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/NilFoundation/nil/nil/internal/contracts"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
	"github.com/NilFoundation/nil/nil/services/txnpool"
)

//go:generate go run github.com/matryer/moq -out client_generated_mock.go -rm -stub -with-resets . Client

type BatchRequest interface {
	GetBlock(shardId types.ShardId, blockId any, fullTx bool) (uint64, error)
	GetDebugBlock(shardId types.ShardId, blockId any, fullTx bool) (uint64, error)
	SendTransactionViaSmartContract(
		ctx context.Context,
		smartAccountAddress types.Address,
		bytecode types.Code,
		fee types.FeePack,
		value types.Value,
		tokens []types.TokenBalance,
		contractAddress types.Address,
		pk *ecdsa.PrivateKey,
	) (uint64, error)
}

// Client defines the interface for a client
// Note: This interface is designed for JSON-RPC. If you need to extend support
// for other protocols like WebSocket or gRPC in the future, you might need to
// change or extend this interface to accommodate those protocols.
type Client interface {
	RawClient
	DbClient
	Web3Client
	TxPoolClient
	jsonrpc.DevAPI

	CreateBatchRequest() BatchRequest
	BatchCall(ctx context.Context, req BatchRequest) ([]any, error)

	EstimateFee(ctx context.Context, args *jsonrpc.CallArgs, blockId any) (*jsonrpc.EstimateFeeRes, error)
	Call(
		ctx context.Context,
		args *jsonrpc.CallArgs,
		blockId any,
		stateOverride *jsonrpc.StateOverrides,
	) (*jsonrpc.CallRes, error)
	GetCode(ctx context.Context, addr types.Address, blockId any) (types.Code, error)
	GetBlock(ctx context.Context, shardId types.ShardId, blockId any, fullTx bool) (*jsonrpc.RPCBlock, error)
	GetBlocksRange(
		ctx context.Context,
		shardId types.ShardId,
		from types.BlockNumber,
		to types.BlockNumber,
		fullTx bool,
		batchSize int,
	) ([]*jsonrpc.RPCBlock, error)
	GetDebugBlock(ctx context.Context, shardId types.ShardId, blockId any, fullTx bool) (*jsonrpc.DebugRPCBlock, error)
	GetDebugBlocksRange(
		ctx context.Context,
		shardId types.ShardId,
		from types.BlockNumber,
		to types.BlockNumber,
		fullTx bool,
		batchSize int,
	) ([]*jsonrpc.DebugRPCBlock, error)
	SendTransaction(ctx context.Context, txn *types.ExternalTransaction) (common.Hash, error)
	SendRawTransaction(ctx context.Context, data []byte) (common.Hash, error)
	GetInTransactionByHash(ctx context.Context, hash common.Hash) (*jsonrpc.RPCInTransaction, error)
	GetInTransactionReceipt(ctx context.Context, hash common.Hash) (*jsonrpc.RPCReceipt, error)
	GetTransactionCount(ctx context.Context, address types.Address, blockId any) (types.Seqno, error)
	GetBlockTransactionCount(ctx context.Context, shardId types.ShardId, blockId any) (uint64, error)
	GetBalance(ctx context.Context, address types.Address, blockId any) (types.Value, error)
	GetShardIdList(ctx context.Context) ([]types.ShardId, error)
	GetNumShards(ctx context.Context) (uint64, error)
	GasPrice(ctx context.Context, shardId types.ShardId) (types.Value, error)
	ChainId(ctx context.Context) (types.ChainId, error)

	DeployContract(
		ctx context.Context, shardId types.ShardId, smartAccountAddress types.Address, payload types.DeployPayload,
		value types.Value, fee types.FeePack, pk *ecdsa.PrivateKey,
	) (common.Hash, types.Address, error)
	DeployExternal(
		ctx context.Context,
		shardId types.ShardId,
		deployPayload types.DeployPayload,
		fee types.FeePack,
	) (common.Hash, types.Address, error)
	SendTransactionViaSmartAccount(
		ctx context.Context,
		smartAccountAddress types.Address,
		bytecode types.Code,
		fee types.FeePack,
		value types.Value,
		tokens []types.TokenBalance,
		contractAddress types.Address,
		pk *ecdsa.PrivateKey,
	) (common.Hash, error)
	SendExternalTransaction(
		ctx context.Context,
		calldata types.Code,
		contractAddress types.Address,
		pk *ecdsa.PrivateKey,
		fee types.FeePack,
	) (common.Hash, error)

	// GetTokens retrieves the contract tokens at the given address
	GetTokens(ctx context.Context, address types.Address, blockId any) (types.TokensMap, error)

	// SetTokenName sets token name
	SetTokenName(
		ctx context.Context, contractAddr types.Address, name string, pk *ecdsa.PrivateKey) (common.Hash, error)

	// ChangeTokenAmount mints / burns token for the contract
	ChangeTokenAmount(
		ctx context.Context,
		contractAddr types.Address,
		amount types.Value,
		pk *ecdsa.PrivateKey,
		mint bool,
	) (common.Hash, error)

	// GetDebugContract retrieves smart contract with its data, such as code, storage and proof
	GetDebugContract(ctx context.Context, contractAddr types.Address, blockId any) (*jsonrpc.DebugRPCContract, error)
}

func EstimateFeeExternal(
	ctx context.Context,
	c Client,
	txn *types.ExternalTransaction,
	blockId any,
) (*jsonrpc.EstimateFeeRes, error) {
	var flags types.TransactionFlags
	if txn.Kind == types.DeployTransactionKind {
		flags = types.NewTransactionFlags(types.TransactionFlagDeploy)
	}

	args := &jsonrpc.CallArgs{
		Data:  (*hexutil.Bytes)(&txn.Data),
		To:    txn.To,
		Flags: flags,
		Seqno: txn.Seqno,
	}

	return c.EstimateFee(ctx, args, blockId)
}

func CreateExternalTransaction(
	ctx context.Context, c Client, bytecode types.Code, contractAddress types.Address,
	fee types.FeePack, isDeploy bool, id int,
) (*types.ExternalTransaction, error) {
	var kind types.TransactionKind
	if isDeploy {
		kind = types.DeployTransactionKind
	} else {
		kind = types.ExecutionTransactionKind
	}

	// Get the sequence number for the smart account
	seqno, err := c.GetTransactionCount(ctx, contractAddress, "pending")
	if err != nil {
		return nil, err
	}
	seqno += types.Seqno(id)

	// Create the transaction with the bytecode to run
	extTxn := &types.ExternalTransaction{
		To:                   contractAddress,
		Data:                 bytecode,
		Seqno:                seqno,
		Kind:                 kind,
		FeeCredit:            fee.FeeCredit,
		MaxPriorityFeePerGas: fee.MaxPriorityFeePerGas,
		MaxFeePerGas:         fee.MaxFeePerGas,
	}

	if fee.FeeCredit.IsZero() {
		var err error
		var estimatedFee *jsonrpc.EstimateFeeRes
		if estimatedFee, err = EstimateFeeExternal(ctx, c, extTxn, "latest"); err != nil {
			return nil, err
		}
		fee.FeeCredit = estimatedFee.FeeCredit
		extTxn.MaxFeePerGas = estimatedFee.MaxBasFee.Add(estimatedFee.AveragePriorityFee)
		extTxn.MaxPriorityFeePerGas = estimatedFee.AveragePriorityFee
	}
	extTxn.FeeCredit = fee.FeeCredit

	return extTxn, nil
}

func SendExternalTransaction(
	ctx context.Context, c Client, calldata types.Code, contractAddress types.Address,
	pk *ecdsa.PrivateKey, fee types.FeePack, isDeploy bool, withRetry bool,
) (common.Hash, error) {
	extTxn, err := CreateExternalTransaction(ctx, c, calldata, contractAddress, fee, isDeploy, 0)
	if err != nil {
		return common.EmptyHash, err
	}

	if withRetry {
		return sendExternalTransactionWithSeqnoRetry(ctx, c, extTxn, pk)
	}

	if pk != nil {
		err = extTxn.Sign(pk)
		if err != nil {
			return common.EmptyHash, err
		}
	}

	return c.SendTransaction(ctx, extTxn)
}

// sendExternalTransactionWithSeqnoRetry tries to send an external transaction increasing seqno if needed.
// Can be used to ensure sending transactions to common contracts like Faucet.
func sendExternalTransactionWithSeqnoRetry(
	ctx context.Context,
	c Client,
	txn *types.ExternalTransaction,
	pk *ecdsa.PrivateKey,
) (common.Hash, error) {
	var err error
	for range 20 {
		if pk != nil {
			if err := txn.Sign(pk); err != nil {
				return common.EmptyHash, err
			}
		}

		var txHash common.Hash
		txHash, err = c.SendTransaction(ctx, txn)
		if err == nil {
			return txHash, nil
		}
		if !strings.Contains(err.Error(), txnpool.NotReplaced.String()) &&
			!strings.Contains(err.Error(), txnpool.SeqnoTooLow.String()) {
			return common.EmptyHash, err
		}

		txn.Seqno++
	}
	return common.EmptyHash, fmt.Errorf("failed to send transaction in 20 retries, getting %w", err)
}

func CreateInternalTransactionPayload(bytecode types.Code, value types.Value,
	tokens []types.TokenBalance, contractAddress types.Address, isDeploy bool,
) (types.Code, error) {
	var err error
	var calldataExt types.Code
	if isDeploy {
		dp := types.ParseDeployPayload(bytecode)
		if dp == nil {
			return types.Code{}, errors.New("failed to parse deploy payload")
		}
		calldataExt, err = contracts.NewCallData(contracts.NameSmartAccount, "asyncDeploy",
			big.NewInt(int64(contractAddress.ShardId())), value.ToBig(), []byte(dp.Code()), dp.Salt().Big())
	} else {
		calldataExt, err = contracts.NewCallData(contracts.NameSmartAccount, "asyncCall",
			contractAddress, types.EmptyAddress, types.EmptyAddress, tokens, value, []byte(bytecode))
	}
	if err != nil {
		return types.Code{}, err
	}

	return calldataExt, err
}

func SendTransactionViaSmartAccount(
	ctx context.Context,
	c Client,
	smartAccountAddress types.Address,
	bytecode types.Code,
	fee types.FeePack,
	value types.Value,
	tokens []types.TokenBalance,
	contractAddress types.Address,
	pk *ecdsa.PrivateKey,
	isDeploy bool,
) (common.Hash, error) {
	calldataExt, err := CreateInternalTransactionPayload(bytecode, value, tokens, contractAddress, isDeploy)
	if err != nil {
		return common.EmptyHash, err
	}
	return c.SendExternalTransaction(ctx, calldataExt, smartAccountAddress, pk, fee)
}

func WaitForReceipt(
	ctx context.Context,
	client Client,
	hash common.Hash,
	timeout time.Duration,
) (*jsonrpc.RPCReceipt, error) {
	var receipt *jsonrpc.RPCReceipt
	var err error
	err = common.WaitFor(ctx, timeout, 500*time.Millisecond, func(ctx context.Context) bool {
		receipt, err = client.GetInTransactionReceipt(ctx, hash)
		return err == nil && receipt.IsComplete()
	})

	return receipt, err
}
