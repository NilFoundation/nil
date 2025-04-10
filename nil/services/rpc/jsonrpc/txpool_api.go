package jsonrpc

import (
	"context"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/rawapi"
)

type TxPoolStatus struct {
	Pending uint64 `json:"pending" ch:"pending"`
	Queued  uint64 `json:"queued" ch:"queued"`
}

type TxPoolContent struct {
	Pending map[string]map[string]*Transaction `json:"pending"`
	Queued  map[string]map[string]*Transaction `json:"queued"`
}

// TxPoolAPI The txpool API gives access to several non-standard RPC methods to inspect the contents of the txpool
type TxPoolAPI interface {
	GetTxpoolStatus(ctx context.Context, shardId types.ShardId) (TxPoolStatus, error)
	GetTxpoolContent(ctx context.Context, shardId types.ShardId) (TxPoolContent, error)
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

// GetTxpoolStatus inspection property can be queried for the number of transactions currently pending for inclusion
// in the next block(s).
func (api *TxPoolAPIImpl) GetTxpoolStatus(ctx context.Context, shardId types.ShardId) (TxPoolStatus, error) {
	getPendingTxCount, err := api.rawApi.GetTxpoolStatus(ctx, shardId)
	if err != nil {
		return TxPoolStatus{}, err
	}
	return TxPoolStatus{Pending: getPendingTxCount, Queued: 0}, nil
}

// GetTxpoolContent inspection property can be queried to list the exact details of all the transactions
// currently pending for inclusion in the next block(s).
func (api *TxPoolAPIImpl) GetTxpoolContent(ctx context.Context, shardId types.ShardId) (TxPoolContent, error) {
	txPool, err := api.rawApi.GetTxpoolContent(ctx, shardId)
	if err != nil {
		return TxPoolContent{}, err
	}
	pendingTx := make(map[string]map[string]*Transaction)

	for _, tx := range txPool {
		fromAddr := tx.From.String()

		if _, exists := pendingTx[fromAddr]; !exists {
			pendingTx[fromAddr] = make(map[string]*Transaction)
		}

		pendingTx[fromAddr][tx.Seqno.String()] = NewTransaction(tx)
	}
	return TxPoolContent{Pending: pendingTx}, nil
}
