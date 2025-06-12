package mpttracer

import (
	"errors"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/types"
)

type TraceableAccount struct {
	execution.AccountState
	initialSlots  execution.Storage
	slotsUpdates  execution.Storage
	initialValues *types.SmartContract
}

var _ execution.AccountState = (*TraceableAccount)(nil)

func NewTracableAccountState(
	dbRwTxProvider execution.DbRwTxProvider,
	addr types.Address,
	account *types.SmartContract,
	logger logging.Logger,
) (*TraceableAccount, error) {
	accountState, err := execution.NewAccountState(dbRwTxProvider, addr, account, logger)
	if err != nil {
		return nil, err
	}
	if account == nil {
		account = &types.SmartContract{CodeHash: types.EmptyCodeHash}
	}
	return &TraceableAccount{
		AccountState:  accountState,
		initialSlots:  make(execution.Storage),
		slotsUpdates:  make(execution.Storage),
		initialValues: account,
	}, nil
}

func (as *TraceableAccount) GetState(key common.Hash) (common.Hash, error) {
	val, err := as.AccountState.GetState(key)
	if err != nil {
		return common.EmptyHash, err
	}
	_, wasAdded := as.initialSlots[key]
	if !wasAdded {
		as.initialSlots[key] = val
	}
	return val, err
}

func (as *TraceableAccount) SetState(key common.Hash, val common.Hash) {
	as.AccountState.SetState(key, val)
	as.slotsUpdates[key] = val
}

func (as *TraceableAccount) Commit() (*types.SmartContract, error) {
	smartContract, err := as.AccountState.Commit()
	if errors.Is(err, db.ErrKeyNotFound) {
		// TODO: currently, whole storage is read during `debug_getContract`, move to pure `eth_getStorageAt` calls.
		// if `db.KeyNotFound`, fetch them with `debug_storageRangeAt`
		panic("not implemented")
	}
	return smartContract, err
}
