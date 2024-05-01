package execution

import (
	"context"
	"github.com/NilFoundation/nil/common"
	db "github.com/NilFoundation/nil/core/db"
	"github.com/holiman/uint256"
	mpt "github.com/keybase/go-merkle-tree"
)

type AccountState struct {
	Tx          db.Tx
	Balance     uint256.Int
	Code        []byte
	StorageRoot *db.MerkleTree

	State map[common.Hash]uint256.Int
}

type ExecutionState struct {
	Tx           db.Tx
	ContractRoot *db.MerkleTree

	Accounts map[common.Address]*AccountState
}

func NewAccountState(Tx db.Tx, account_hash common.Hash) *AccountState {
	account := db.ReadContract(Tx, account_hash)

	root := db.GetMerkleTree(Tx, account.StorageRoot)
	return &AccountState{Tx: Tx, StorageRoot: root}
}

func NewExecutionState(Tx db.Tx, block_hash common.Hash) (*ExecutionState, error) {
	block := db.ReadBlock(Tx, block_hash)
	root := db.GetMerkleTree(Tx, block.SmartContractsRoot)

	return &ExecutionState{Tx: Tx, ContractRoot: root}, nil
}

func (es *ExecutionState) GetAccount(addr common.Address) (*AccountState, error) {
	acc, ok := es.Accounts[addr]
	if ok {
		return acc, nil
	}

	addr_hash := addr.Hash()
	acc_hash, err := es.ContractRoot.Find(addr_hash)
	if err != nil {
		return nil, err
	}

	acc = NewAccountState(es.Tx, acc_hash.(common.Hash))

	return acc, nil
}

func (as *AccountState) ReadStorage(key common.Hash) (uint256.Int, error) {
	val, ok := as.State[key]
	if ok {
		return val, nil
	}

	raw_val, err := as.StorageRoot.Find(key)

	if err != nil {
		return *uint256.NewInt(0), err
	}

	val = raw_val.(uint256.Int)

	as.State[key] = val

	return val, nil
}

func (as *AccountState) WriteStorage(key common.Hash, val uint256.Int) {
	as.State[key] = val
}

func (as *AccountState) Commit() (common.Hash, error) {
	for k, v := range as.State {
		err := as.StorageRoot.Upsert(k, v)
		if err != nil {
			return common.Hash{}, err
		}
	}

	tree := as.StorageRoot

	return tree.Root()
}

func (es *ExecutionState) ReadStorage(addr common.Address, key common.Hash) (uint256.Int, error) {
	acc, err := es.GetAccount(addr)
	if err != nil {
		return *uint256.NewInt(0), err
	}

	return acc.ReadStorage(key)
}

func (es *ExecutionState) WriteStorage(addr common.Address, key common.Hash, val uint256.Int) error {
	acc, err := es.GetAccount(addr)
	if err != nil {
		return err
	}

	acc.WriteStorage(key, val)
	return nil
}
