package internal

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

var _ shardApiRw = (*shardApiClientRw)(nil)

func constructShardApiClientRw(performer shardApiRequestPerformer) *shardApiClientRw {
	return &shardApiClientRw{
		shardApiRequestPerformer: performer,
	}
}

func newShardApiClientNetworkRw(shardId types.ShardId, networkManager network.Manager) *shardApiClientRw {
	client, err := newShardApiClientNetwork[shardApiClientRw, shardApiRw, NetworkTransportProtocolRw](
		constructShardApiClientRw, shardId, apiNameRw, networkManager)
	check.PanicIfErr(err)
	return client
}

func newShardApiClientDirectEmulatorRw(shardApi shardApiRw) *shardApiClientRw {
	client, err := newShardApiClientDirectEmulator[shardApiClientRw, shardApiRw, NetworkTransportProtocolRw](
		constructShardApiClientRw, apiNameRw, shardApi)
	check.PanicIfErr(err)
	return client
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
