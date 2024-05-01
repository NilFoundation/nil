package execution

import (
	"errors"
	"github.com/NilFoundation/nil/common"
	db "github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/types"
	"github.com/holiman/uint256"
)

type AccountState struct {
	Tx          db.Tx
	Balance     uint256.Int
	Code        types.Code
	CodeHash    common.Hash
	StorageRoot *db.MerkleTree

	State map[common.Hash]uint256.Int
}

type ExecutionState struct {
	Tx           db.Tx
	ContractRoot *db.MerkleTree
	PrevBlock    common.Hash

	Accounts map[common.Hash]*AccountState
}

func NewAccountState(Tx db.Tx, account_hash common.Hash) (*AccountState, error) {
	account := db.ReadContract(Tx, account_hash)

	root := db.GetMerkleTree(Tx, account.StorageRoot)

	code, err := db.ReadCode(Tx, account.CodeHash)

	if err != nil {
		return nil, err
	}

	if code == nil {
		return nil, errors.New("Cannot retrieve code")
	}

	return &AccountState{Tx: Tx, StorageRoot: root, CodeHash: account.CodeHash, Code: *code}, nil
}

func NewExecutionState(Tx db.Tx, block_hash common.Hash) (*ExecutionState, error) {
	block := db.ReadBlock(Tx, block_hash)

	root := db.GetMerkleTree(Tx, common.Hash{})
	if block != nil {
		root = db.GetMerkleTree(Tx, block.SmartContractsRoot)
	}

	return &ExecutionState{Tx: Tx, ContractRoot: root, PrevBlock: block_hash}, nil
}

func (es *ExecutionState) GetAccount(addr common.Hash) (*AccountState, error) {
	acc, ok := es.Accounts[addr]
	if ok {
		return acc, nil
	}

	acc_hash, err := es.ContractRoot.Find(addr)
	if err != nil {
		return nil, err
	}

	return NewAccountState(es.Tx, acc_hash.(common.Hash))
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

	storageRoot, err := as.StorageRoot.Root()

	if err != nil {
		return common.Hash{}, err
	}

	acc := types.SmartContract{
		Balance:     as.Balance,
		StorageRoot: storageRoot,
		CodeHash:    as.CodeHash,
	}

	acc_hash := acc.Hash()

	err = db.WriteContract(as.Tx, &acc)

	if err != nil {
		return common.Hash{}, err
	}

	return acc_hash, nil
}

func (es *ExecutionState) ReadStorage(addr common.Hash, key common.Hash) (uint256.Int, error) {
	acc, err := es.GetAccount(addr)
	if err != nil {
		return *uint256.NewInt(0), err
	}

	return acc.ReadStorage(key)
}

func (es *ExecutionState) WriteStorage(addr common.Hash, key common.Hash, val uint256.Int) error {
	acc, err := es.GetAccount(addr)
	if err != nil {
		return err
	}

	acc.WriteStorage(key, val)
	return nil
}

func (es *ExecutionState) CreateContract(addr common.Hash, code types.Code) error {
	acc, err := es.GetAccount(addr)

	if err != nil {
		return err
	}

	if acc != nil {
		return errors.New("Contract already exists")
	}

	root := db.GetMerkleTree(es.Tx, common.Hash{})

	new_acc := AccountState{
		Tx:          es.Tx,
		StorageRoot: root,
		CodeHash:    code.Hash(),
		Code:        code,
	}

	es.Accounts[addr] = &new_acc

	return nil
}

func (es *ExecutionState) Commit() (common.Hash, error) {
	for k, acc := range es.Accounts {
		v, err := acc.Commit()
		if err != nil {
			return common.Hash{}, err
		}
		err = es.ContractRoot.Upsert(k, v)
		if err != nil {
			return common.Hash{}, err
		}
	}

	contractRoot, err := es.ContractRoot.Root()

	if err != nil {
		return common.Hash{}, err
	}

	block := types.Block{
		Id:                 0,
		PrevBlock:          es.PrevBlock,
		SmartContractsRoot: contractRoot,
	}

	block_hash := block.Hash()

	err = db.WriteBlock(es.Tx, &block)
	if err != nil {
		return common.Hash{}, err
	}

	return block_hash, nil
}
