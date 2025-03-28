package rawapi

import (
	"context"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/common/sszx"
	"github.com/NilFoundation/nil/nil/internal/network"
	"github.com/NilFoundation/nil/nil/internal/types"
	rawapitypes "github.com/NilFoundation/nil/nil/services/rpc/rawapi/types"
	rpctypes "github.com/NilFoundation/nil/nil/services/rpc/types"
	"github.com/NilFoundation/nil/nil/services/txnpool"
)

type NodeApiRo interface {
	GetBlockHeader(
		ctx context.Context,
		shardId types.ShardId,
		blockReference rawapitypes.BlockReference,
	) (sszx.SSZEncodedData, error)
	GetFullBlockData(
		ctx context.Context,
		shardId types.ShardId,
		blockReference rawapitypes.BlockReference,
	) (*types.RawBlockWithExtractedData, error)
	GetBlockTransactionCount(
		ctx context.Context, shardId types.ShardId, blockReference rawapitypes.BlockReference) (uint64, error)

	GetInTransaction(
		ctx context.Context,
		shardId types.ShardId,
		transactionRequest rawapitypes.TransactionRequest,
	) (*rawapitypes.TransactionInfo, error)
	GetInTransactionReceipt(
		ctx context.Context, shardId types.ShardId, hash common.Hash) (*rawapitypes.ReceiptInfo, error)

	GetBalance(
		ctx context.Context, address types.Address, blockReference rawapitypes.BlockReference) (types.Value, error)
	GetCode(ctx context.Context, address types.Address, blockReference rawapitypes.BlockReference) (types.Code, error)
	GetTokens(
		ctx context.Context,
		address types.Address,
		blockReference rawapitypes.BlockReference,
	) (map[types.TokenId]types.Value, error)
	GetTransactionCount(
		ctx context.Context, address types.Address, blockReference rawapitypes.BlockReference) (uint64, error)
	GetContract(
		ctx context.Context,
		address types.Address,
		blockReference rawapitypes.BlockReference,
	) (*rawapitypes.SmartContract, error)

	Call(
		ctx context.Context,
		args rpctypes.CallArgs,
		mainBlockReferenceOrHashWithChildren rawapitypes.BlockReferenceOrHashWithChildren,
		overrides *rpctypes.StateOverrides,
	) (*rpctypes.CallResWithGasPrice, error)

	GasPrice(ctx context.Context, shardId types.ShardId) (types.Value, error)
	GetShardIdList(ctx context.Context) ([]types.ShardId, error)
	GetNumShards(ctx context.Context) (uint64, error)

	ClientVersion(ctx context.Context) (string, error)

	GetTxpoolStatus(ctx context.Context, shardId types.ShardId) (uint64, error)
	GetTxpoolContent(ctx context.Context, shardId types.ShardId) ([]*types.Transaction, error)
}

type NodeApi interface {
	NodeApiRo
	SendTransaction(ctx context.Context, shardId types.ShardId, transaction []byte) (txnpool.DiscardReason, error)
	DoPanicOnShard(ctx context.Context, shardId types.ShardId) (uint64, error)
}

type ShardApiRo interface {
	GetBlockHeader(ctx context.Context, blockReference rawapitypes.BlockReference) (sszx.SSZEncodedData, error)
	GetFullBlockData(
		ctx context.Context, blockReference rawapitypes.BlockReference) (*types.RawBlockWithExtractedData, error)
	GetBlockTransactionCount(ctx context.Context, blockReference rawapitypes.BlockReference) (uint64, error)

	GetInTransaction(
		ctx context.Context, transactionRequest rawapitypes.TransactionRequest) (*rawapitypes.TransactionInfo, error)
	GetInTransactionReceipt(ctx context.Context, hash common.Hash) (*rawapitypes.ReceiptInfo, error)

	GetBalance(
		ctx context.Context, address types.Address, blockReference rawapitypes.BlockReference) (types.Value, error)
	GetCode(ctx context.Context, address types.Address, blockReference rawapitypes.BlockReference) (types.Code, error)
	GetTokens(
		ctx context.Context,
		address types.Address,
		blockReference rawapitypes.BlockReference,
	) (map[types.TokenId]types.Value, error)
	GetTransactionCount(
		ctx context.Context, address types.Address, blockReference rawapitypes.BlockReference) (uint64, error)
	GetContract(
		ctx context.Context,
		address types.Address,
		blockReference rawapitypes.BlockReference,
	) (*rawapitypes.SmartContract, error)

	Call(
		ctx context.Context,
		args rpctypes.CallArgs,
		mainBlockReferenceOrHashWithChildren rawapitypes.BlockReferenceOrHashWithChildren,
		overrides *rpctypes.StateOverrides,
	) (*rpctypes.CallResWithGasPrice, error)

	GasPrice(ctx context.Context) (types.Value, error)
	GetShardIdList(ctx context.Context) ([]types.ShardId, error)
	GetNumShards(ctx context.Context) (uint64, error)

	ClientVersion(ctx context.Context) (string, error)

	setAsP2pRequestHandlersIfAllowed(
		ctx context.Context, networkManager *network.Manager, readonly bool, logger logging.Logger) error

	GetTxpoolStatus(ctx context.Context) (uint64, error)
	GetTxpoolContent(ctx context.Context) ([]*types.Transaction, error)

	setNodeApi(nodeApi NodeApi)
}

type ShardApi interface {
	ShardApiRo
	SendTransaction(ctx context.Context, transaction []byte) (txnpool.DiscardReason, error)
	DoPanicOnShard(ctx context.Context) (uint64, error)
}

func SetShardApiAsP2pRequestHandlersIfAllowed(
	shardApi ShardApi,
	ctx context.Context,
	networkManager *network.Manager,
	readonly bool,
	logger logging.Logger,
) error {
	return shardApi.setAsP2pRequestHandlersIfAllowed(ctx, networkManager, readonly, logger)
}
