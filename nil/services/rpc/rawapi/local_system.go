package rawapi

import (
	"context"
	"errors"
	"fmt"

	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/types"
)

func (api *LocalShardApi) GasPrice(ctx context.Context) (types.Value, error) {
	tx, err := api.db.CreateRoTx(ctx)
	if err != nil {
		return types.Value{}, fmt.Errorf("cannot open tx: %w", err)
	}
	defer tx.Rollback()

	gasPrice, err := db.ReadGasPerShard(tx, api.ShardId)
	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			return types.NewValueFromUint64(0), nil
		}
		return types.Value{}, err
	}

	return gasPrice, nil
}
