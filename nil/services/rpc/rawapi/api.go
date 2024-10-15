package rawapi

import (
	"context"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/ssz"
	"github.com/NilFoundation/nil/nil/internal/types"
	rawapitypes "github.com/NilFoundation/nil/nil/services/rpc/rawapi/types"
	rpctypes "github.com/NilFoundation/nil/nil/services/rpc/types"
)

type NodeApi interface {
	GetBlockHeader(ctx context.Context, shardId types.ShardId, blockReference rawapitypes.BlockReference) (ssz.SSZEncodedData, error)
	GetFullBlockData(ctx context.Context, shardId types.ShardId, blockReference rawapitypes.BlockReference) (*types.RawBlockWithExtractedData, error)
	GetBlockTransactionCount(ctx context.Context, shardId types.ShardId, blockReference rawapitypes.BlockReference) (uint64, error)

	GetInMessage(ctx context.Context, shardId types.ShardId, messageRequest rawapitypes.MessageRequest) (*rawapitypes.MessageInfo, error)
	GetInMessageReceipt(ctx context.Context, shardId types.ShardId, hash common.Hash) (*rawapitypes.ReceiptInfo, error)

	GetBalance(ctx context.Context, address types.Address, blockReference rawapitypes.BlockReference) (types.Value, error)
	GetCode(ctx context.Context, address types.Address, blockReference rawapitypes.BlockReference) (types.Code, error)
	GetCurrencies(ctx context.Context, address types.Address, blockReference rawapitypes.BlockReference) (map[types.CurrencyId]types.Value, error)
	Call(
		ctx context.Context, args rpctypes.CallArgs, mainBlockNrOrHash rawapitypes.BlockReference, overrides *rpctypes.StateOverrides, emptyMessageIsRoot bool,
	) (*rpctypes.CallResWithGasPrice, error)

	GasPrice(ctx context.Context, shardId types.ShardId) (types.Value, error)
	GetShardIdList(ctx context.Context) ([]types.ShardId, error)
}

type ShardApi interface {
	GetBlockHeader(ctx context.Context, blockReference rawapitypes.BlockReference) (ssz.SSZEncodedData, error)
	GetFullBlockData(ctx context.Context, blockReference rawapitypes.BlockReference) (*types.RawBlockWithExtractedData, error)
	GetBlockTransactionCount(ctx context.Context, blockReference rawapitypes.BlockReference) (uint64, error)

	GetInMessage(ctx context.Context, messageRequest rawapitypes.MessageRequest) (*rawapitypes.MessageInfo, error)
	GetInMessageReceipt(ctx context.Context, hash common.Hash) (*rawapitypes.ReceiptInfo, error)

	GetBalance(ctx context.Context, address types.Address, blockReference rawapitypes.BlockReference) (types.Value, error)
	GetCode(ctx context.Context, address types.Address, blockReference rawapitypes.BlockReference) (types.Code, error)
	GetCurrencies(ctx context.Context, address types.Address, blockReference rawapitypes.BlockReference) (map[types.CurrencyId]types.Value, error)

	Call(
		ctx context.Context, args rpctypes.CallArgs, mainBlockNrOrHash rawapitypes.BlockReference, overrides *rpctypes.StateOverrides, emptyMessageIsRoot bool,
	) (*rpctypes.CallResWithGasPrice, error)

	GasPrice(ctx context.Context) (types.Value, error)
	GetShardIdList(ctx context.Context) ([]types.ShardId, error)
}
