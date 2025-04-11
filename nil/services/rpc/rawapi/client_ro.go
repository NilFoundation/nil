package rawapi

import (
	"context"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/common/sszx"
	"github.com/NilFoundation/nil/nil/internal/network"
	"github.com/NilFoundation/nil/nil/internal/types"
	rawapitypes "github.com/NilFoundation/nil/nil/services/rpc/rawapi/types"
	rpctypes "github.com/NilFoundation/nil/nil/services/rpc/types"
)

type shardApiClientRo struct {
	shardApiRequestPerformer
}

var (
	_ shardApiRequestPerformerSetter = (*shardApiClientRo)(nil)
	_ ShardApiRo                     = (*shardApiClientRo)(nil)
)

func newShardApiClientNetworkRo(shardId types.ShardId, networkManager network.Manager) *shardApiClientRo {
	client, err := newShardApiClientNetwork[ShardApiRo, NetworkTransportProtocolRo, *shardApiClientRo](
		shardId, apiNameRo, networkManager)
	check.PanicIfErr(err)
	return client
}

func newShardApiClientDirectEmulatorRo(shardApi ShardApiRo) *shardApiClientRo {
	client, err := newShardApiClientDirectEmulator[ShardApiRo, NetworkTransportProtocolRo, *shardApiClientRo](
		apiNameRo, shardApi)
	check.PanicIfErr(err)
	return client
}

func (api *shardApiClientRo) setShardApiRequestPerformer(requestPerformer shardApiRequestPerformer) {
	api.shardApiRequestPerformer = requestPerformer
}

func (api *shardApiClientRo) GetBlockHeader(
	ctx context.Context, blockReference rawapitypes.BlockReference,
) (sszx.SSZEncodedData, error) {
	return sendRequestAndGetResponseWithCallerMethodName[sszx.SSZEncodedData](
		ctx, api.shardApiRequestPerformer, "GetBlockHeader", blockReference)
}

func (api *shardApiClientRo) GetFullBlockData(
	ctx context.Context, blockReference rawapitypes.BlockReference,
) (*types.RawBlockWithExtractedData, error) {
	return sendRequestAndGetResponseWithCallerMethodName[*types.RawBlockWithExtractedData](
		ctx, api.shardApiRequestPerformer, "GetFullBlockData", blockReference)
}

func (api *shardApiClientRo) GetBlockTransactionCount(
	ctx context.Context, blockReference rawapitypes.BlockReference,
) (uint64, error) {
	return sendRequestAndGetResponseWithCallerMethodName[uint64](
		ctx, api, "GetBlockTransactionCount", blockReference)
}

func (api *shardApiClientRo) GetBalance(
	ctx context.Context, address types.Address, blockReference rawapitypes.BlockReference,
) (types.Value, error) {
	return sendRequestAndGetResponseWithCallerMethodName[types.Value](
		ctx, api, "GetBalance", address, blockReference)
}

func (api *shardApiClientRo) GetCode(
	ctx context.Context, address types.Address, blockReference rawapitypes.BlockReference,
) (types.Code, error) {
	return sendRequestAndGetResponseWithCallerMethodName[types.Code](
		ctx, api, "GetCode", address, blockReference)
}

func (api *shardApiClientRo) GetTokens(
	ctx context.Context, address types.Address, blockReference rawapitypes.BlockReference,
) (map[types.TokenId]types.Value, error) {
	return sendRequestAndGetResponseWithCallerMethodName[map[types.TokenId]types.Value](
		ctx, api, "GetTokens", address, blockReference)
}

func (api *shardApiClientRo) GetContract(
	ctx context.Context, address types.Address, blockReference rawapitypes.BlockReference,
) (*rawapitypes.SmartContract, error) {
	return sendRequestAndGetResponseWithCallerMethodName[*rawapitypes.SmartContract](
		ctx, api, "GetContract", address, blockReference)
}

func (api *shardApiClientRo) Call(
	ctx context.Context,
	args rpctypes.CallArgs,
	mainBlockReferenceOrHashWithChildren rawapitypes.BlockReferenceOrHashWithChildren,
	overrides *rpctypes.StateOverrides,
) (*rpctypes.CallResWithGasPrice, error) {
	return sendRequestAndGetResponseWithCallerMethodName[*rpctypes.CallResWithGasPrice](
		ctx, api, "Call", args, mainBlockReferenceOrHashWithChildren, overrides)
}

func (api *shardApiClientRo) GetInTransaction(
	ctx context.Context, request rawapitypes.TransactionRequest,
) (*rawapitypes.TransactionInfo, error) {
	return sendRequestAndGetResponseWithCallerMethodName[*rawapitypes.TransactionInfo](
		ctx, api, "GetInTransaction", request)
}

func (api *shardApiClientRo) GetInTransactionReceipt(
	ctx context.Context, hash common.Hash,
) (*rawapitypes.ReceiptInfo, error) {
	return sendRequestAndGetResponseWithCallerMethodName[*rawapitypes.ReceiptInfo](
		ctx, api, "GetInTransactionReceipt", hash)
}

func (api *shardApiClientRo) GasPrice(ctx context.Context) (types.Value, error) {
	return sendRequestAndGetResponseWithCallerMethodName[types.Value](ctx, api, "GasPrice")
}

func (api *shardApiClientRo) GetShardIdList(ctx context.Context) ([]types.ShardId, error) {
	return sendRequestAndGetResponseWithCallerMethodName[[]types.ShardId](ctx, api, "GetShardIdList")
}

func (api *shardApiClientRo) GetNumShards(ctx context.Context) (uint64, error) {
	return sendRequestAndGetResponseWithCallerMethodName[uint64](ctx, api, "GetNumShards")
}

func (api *shardApiClientRo) ClientVersion(ctx context.Context) (string, error) {
	return sendRequestAndGetResponseWithCallerMethodName[string](ctx, api, "ClientVersion")
}
