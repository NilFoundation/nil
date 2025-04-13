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

type shardApiBase interface {
	shardId() types.ShardId
	setNodeApi(nodeApi NodeApi)
	setAsP2pRequestHandlersIfAllowed(ctx context.Context, networkManager network.Manager, logger logging.Logger) error
}

const apiNameRo = "rawapi_ro"

type ShardApiRo interface {
	shardApiBase

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
}

const apiNameRw = "rawapi_rw"

type ShardApiRw interface {
	shardApiBase

	SendTransaction(ctx context.Context, transaction []byte) (txnpool.DiscardReason, error)
	GetTransactionCount(
		ctx context.Context, address types.Address, blockReference rawapitypes.BlockReference) (uint64, error)

	GetTxpoolStatus(ctx context.Context) (uint64, error)
	GetTxpoolContent(ctx context.Context) ([]*types.Transaction, error)
}

const apiNameDev = "rawapi_dev"

type ShardApiDev interface {
	shardApiBase

	DoPanicOnShard(ctx context.Context) (uint64, error)
}
