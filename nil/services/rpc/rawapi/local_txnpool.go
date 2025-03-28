package rawapi

import (
	"context"
	"errors"
	"fmt"

	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/txnpool"
)

func (api *LocalShardApi) SendTransaction(ctx context.Context, encoded []byte) (txnpool.DiscardReason, error) {
	if api.txnpool == nil {
		return 0, errors.New("transaction pool is not available")
	}

	var extTxn types.ExternalTransaction
	if err := extTxn.UnmarshalSSZ(encoded); err != nil {
		return 0, fmt.Errorf("failed to decode transaction: %w", err)
	}

	reasons, err := api.txnpool.Add(ctx, extTxn.ToTransaction())
	if err != nil {
		return 0, err
	}
	return reasons[0], nil
}

func (api *LocalShardApi) GetTxpoolStatus(ctx context.Context) (uint64, error) {
	return uint64(api.txnpool.GetQueue().Len()), nil
}

func (api *LocalShardApi) GetTxpoolContent(ctx context.Context) ([]*types.Transaction, error) {
	txnWithHash, err := api.txnpool.Peek(api.txnpool.GetQueue().Len())
	if err != nil {
		return nil, err
	}
	res := make([]*types.Transaction, 0, len(txnWithHash))
	for _, txn := range txnWithHash {
		res = append(res, txn.Transaction)
	}
	return res, nil
}
