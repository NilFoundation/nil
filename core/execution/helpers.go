package execution

import (
	"context"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/types"
)

type TxOwner struct {
	Ctx  context.Context
	RoTx db.RoTx
	RwTx db.RwTx
}

func NewTxOwner(ctx context.Context, txFabric db.DB) (*TxOwner, error) {
	roTx, err := txFabric.CreateRoTx(ctx)
	if err != nil {
		return nil, err
	}
	rwTx, err := txFabric.CreateRwTx(ctx)
	if err != nil {
		return nil, err
	}
	return &TxOwner{
		Ctx:  ctx,
		RoTx: roTx,
		RwTx: rwTx,
	}, nil
}

func (t *TxOwner) Rollback() {
	t.Ctx = nil
	if t.RoTx != nil {
		t.RoTx.Rollback()
		t.RoTx = nil
	}
	if t.RwTx != nil {
		t.RwTx.Rollback()
		t.RwTx = nil
	}
}

func (t *TxOwner) Commit() error {
	if err := t.RwTx.Commit(); err != nil {
		return err
	}
	t.Rollback()
	return nil
}

func NewErrorLog(addr types.Address, err error) *types.Log {
	return types.NewLog(addr, []byte(err.Error()), []common.Hash{types.TopicErrorMessage})
}
