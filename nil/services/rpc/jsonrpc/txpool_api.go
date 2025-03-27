package jsonrpc

import (
	"context"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/rawapi"
)

type TxPoolAPI interface {
	GetTxpoolStatus(ctx context.Context, shardId types.ShardId) (uint64, error)
	GetTxpoolContent(ctx context.Context, shardId types.ShardId) ([]*types.TxnWithHash, error)
}

type TxPoolAPIImpl struct {
	logger logging.Logger
	rawApi rawapi.NodeApi
}

var _ TxPoolAPI = &TxPoolAPIImpl{}

func NewTxPoolAPI(rawApi rawapi.NodeApi, logger logging.Logger) *TxPoolAPIImpl {
	return &TxPoolAPIImpl{
		logger: logger,
		rawApi: rawApi,
	}
}

func (api *TxPoolAPIImpl) GetTxpoolStatus(ctx context.Context, shardId types.ShardId) (uint64, error) {
	return api.rawApi.GetTxpoolStatus(ctx, shardId)
}

func (api *TxPoolAPIImpl) GetTxpoolContent(ctx context.Context, shardId types.ShardId) ([]*types.TxnWithHash, error) {
	return api.rawApi.GetTxpoolContent(ctx, shardId)
}
