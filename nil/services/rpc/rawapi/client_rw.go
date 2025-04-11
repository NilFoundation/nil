package rawapi

import (
	"context"

	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/internal/network"
	"github.com/NilFoundation/nil/nil/internal/types"
	rawapitypes "github.com/NilFoundation/nil/nil/services/rpc/rawapi/types"
	"github.com/NilFoundation/nil/nil/services/txnpool"
)

type shardApiClientRw struct {
	shardApiRequestPerformer
}

var (
	_ ShardApiRw                     = (*shardApiClientRw)(nil)
	_ shardApiRequestPerformerSetter = (*shardApiClientRw)(nil)
)

func newShardApiClientNetworkRw(shardId types.ShardId, networkManager network.Manager) *shardApiClientRw {
	client, err := newShardApiClientNetwork[ShardApiRw, NetworkTransportProtocolRw, *shardApiClientRw](
		shardId, apiNameRw, networkManager)
	check.PanicIfErr(err)
	return client
}

func newShardApiClientDirectEmulatorRw(shardApi ShardApiRw) *shardApiClientRw {
	client, err := newShardApiClientDirectEmulator[ShardApiRw, NetworkTransportProtocolRw, *shardApiClientRw](
		apiNameRw, shardApi)
	check.PanicIfErr(err)
	return client
}

func (api *shardApiClientRw) setShardApiRequestPerformer(requestPerformer shardApiRequestPerformer) {
	api.shardApiRequestPerformer = requestPerformer
}

func (api *shardApiClientRw) SendTransaction(ctx context.Context, transaction []byte) (txnpool.DiscardReason, error) {
	return sendRequestAndGetResponseWithCallerMethodName[txnpool.DiscardReason](
		ctx, api, "SendTransaction", transaction)
}

func (api *shardApiClientRw) GetTransactionCount(
	ctx context.Context, address types.Address, blockReference rawapitypes.BlockReference,
) (uint64, error) {
	return sendRequestAndGetResponseWithCallerMethodName[uint64](
		ctx, api, "GetTransactionCount", address, blockReference)
}

func (api *shardApiClientRw) GetTxpoolStatus(ctx context.Context) (uint64, error) {
	return sendRequestAndGetResponseWithCallerMethodName[uint64](ctx, api, "GetTxpoolStatus")
}

func (api *shardApiClientRw) GetTxpoolContent(ctx context.Context) ([]*types.Transaction, error) {
	return sendRequestAndGetResponseWithCallerMethodName[[]*types.Transaction](ctx, api, "GetTxpoolContent")
}
