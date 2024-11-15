package rawapi

import (
	"context"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/ssz"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/msgpool"
	rawapitypes "github.com/NilFoundation/nil/nil/services/rpc/rawapi/types"
	rpctypes "github.com/NilFoundation/nil/nil/services/rpc/types"
)

type NodeApiRo interface {
	GetBlockHeader(ctx context.Context, shardId types.ShardId, blockReference rawapitypes.BlockReference) (ssz.SSZEncodedData, error)
	GetFullBlockData(ctx context.Context, shardId types.ShardId, blockReference rawapitypes.BlockReference) (*types.RawBlockWithExtractedData, error)
	GetBlockTransactionCount(ctx context.Context, shardId types.ShardId, blockReference rawapitypes.BlockReference) (uint64, error)

	GetInMessage(ctx context.Context, shardId types.ShardId, messageRequest rawapitypes.MessageRequest) (*rawapitypes.MessageInfo, error)
	GetInMessageReceipt(ctx context.Context, shardId types.ShardId, hash common.Hash) (*rawapitypes.ReceiptInfo, error)

	GetBalance(ctx context.Context, address types.Address, blockReference rawapitypes.BlockReference) (types.Value, error)
	GetCode(ctx context.Context, address types.Address, blockReference rawapitypes.BlockReference) (types.Code, error)
	GetCurrencies(ctx context.Context, address types.Address, blockReference rawapitypes.BlockReference) (map[types.CurrencyId]types.Value, error)
	GetMessageCount(ctx context.Context, address types.Address, blockReference rawapitypes.BlockReference) (uint64, error)
	GetContract(ctx context.Context, address types.Address, blockReference rawapitypes.BlockReference) (*rawapitypes.SmartContract, error)

	Call(
		ctx context.Context, args rpctypes.CallArgs, mainBlockReferenceOrHashWithChildren rawapitypes.BlockReferenceOrHashWithChildren, overrides *rpctypes.StateOverrides, emptyMessageIsRoot bool,
	) (*rpctypes.CallResWithGasPrice, error)

	GasPrice(ctx context.Context, shardId types.ShardId) (types.Value, error)
	GetShardIdList(ctx context.Context) ([]types.ShardId, error)
}

type NodeApi interface {
	NodeApiRo
	SendMessage(ctx context.Context, shardId types.ShardId, message []byte) (msgpool.DiscardReason, error)
}

type ShardApiRo interface {
	GetBlockHeader(ctx context.Context, blockReference rawapitypes.BlockReference) (ssz.SSZEncodedData, error)
	GetFullBlockData(ctx context.Context, blockReference rawapitypes.BlockReference) (*types.RawBlockWithExtractedData, error)
	GetBlockTransactionCount(ctx context.Context, blockReference rawapitypes.BlockReference) (uint64, error)

	GetInMessage(ctx context.Context, messageRequest rawapitypes.MessageRequest) (*rawapitypes.MessageInfo, error)
	GetInMessageReceipt(ctx context.Context, hash common.Hash) (*rawapitypes.ReceiptInfo, error)

	GetBalance(ctx context.Context, address types.Address, blockReference rawapitypes.BlockReference) (types.Value, error)
	GetCode(ctx context.Context, address types.Address, blockReference rawapitypes.BlockReference) (types.Code, error)
	GetCurrencies(ctx context.Context, address types.Address, blockReference rawapitypes.BlockReference) (map[types.CurrencyId]types.Value, error)
	GetMessageCount(ctx context.Context, address types.Address, blockReference rawapitypes.BlockReference) (uint64, error)
	GetContract(ctx context.Context, address types.Address, blockReference rawapitypes.BlockReference) (*rawapitypes.SmartContract, error)

	Call(
		ctx context.Context, args rpctypes.CallArgs, mainBlockReferenceOrHashWithChildren rawapitypes.BlockReferenceOrHashWithChildren, overrides *rpctypes.StateOverrides, emptyMessageIsRoot bool,
	) (*rpctypes.CallResWithGasPrice, error)

	GasPrice(ctx context.Context) (types.Value, error)
	GetShardIdList(ctx context.Context) ([]types.ShardId, error)

	setNodeApi(nodeApi NodeApi)
}

type ShardApi interface {
	ShardApiRo
	SendMessage(ctx context.Context, message []byte) (msgpool.DiscardReason, error)
}
