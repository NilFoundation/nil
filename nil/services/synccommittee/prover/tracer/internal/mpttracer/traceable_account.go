package mpttracer

import (
	"github.com/NilFoundation/nil/nil/client"
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/types"
)

type TraceableAccount struct {
	execution.AccountState
	client        client.Client
	initialSlots  execution.Storage
	slotsUpdates  execution.Storage
	initialValues *types.SmartContract
}

var _ execution.AccountState = (*TraceableAccount)(nil)

func NewTracableAccountState(dbRwTxProvider execution.DbRwTxProvider, addr types.Address, account *types.SmartContract, logger logging.Logger) (*TraceableAccount, error) {
	accountState, err := execution.NewAccountState(dbRwTxProvider, addr, account, logger)
	if err != nil {
		return nil, err
	}
	return &TraceableAccount{
		AccountState:  accountState,
		initialSlots:  make(execution.Storage),
		slotsUpdates:  make(execution.Storage),
		initialValues: account,
	}, nil
}

func (as *TraceableAccount) GetState(key common.Hash) (common.Hash, error) {
	as.client.GetStorageAt()
	val, err := as.AccountState.GetState(key)
	if err != nil {
		return common.EmptyHash, err
	}
	_, wasAdded := as.initialSlots[key]
	if !wasAdded {
		as.initialSlots[key] = val
	}
	panic("should fail here to check if overriden read")
}

func (as *TraceableAccount) SetState(key common.Hash, val common.Hash) error {
	if err := as.AccountState.SetState(key, val); err != nil {
		return err
	}
	as.slotsUpdates[key] = val
	panic("should fail here to check if overriden write")
}
