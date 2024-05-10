package execution

import (
	"errors"

	"github.com/NilFoundation/nil/common"
	db "github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/mpt"
	"github.com/NilFoundation/nil/core/ssz"
	"github.com/NilFoundation/nil/core/types"
	"github.com/holiman/uint256"
)

var logger = common.NewLogger("execution", false /* noColor */)

type AccountState struct {
	Tx          db.Tx
	Balance     uint256.Int
	Code        types.Code
	CodeHash    common.Hash
	StorageRoot *mpt.MerklePatriciaTrie

	State map[common.Hash]uint256.Int
}

type ExecutionState struct {
	Tx           db.Tx
	ContractRoot *mpt.MerklePatriciaTrie
	PrevBlock    common.Hash

	Accounts map[common.Address]*AccountState
}

func NewAccountState(tx db.Tx, accountHash common.Hash) (*AccountState, error) {
	account := db.ReadContract(tx, accountHash)

	// TODO: store storage of each contract in separate table
	root := mpt.NewMerklePatriciaTrieWithRoot(tx, db.StorageTrieTable, account.StorageRoot)

	code, err := db.ReadCode(tx, account.CodeHash)

	if err != nil {
		return nil, err
	}

	if code == nil {
		return nil, errors.New("cannot retrieve code")
	}

	return &AccountState{
		Tx:          tx,
		StorageRoot: root,
		CodeHash:    account.CodeHash,
		Code:        *code,
		State:       map[common.Hash]uint256.Int{},
	}, nil
}

func NewExecutionState(tx db.Tx, blockHash common.Hash) (*ExecutionState, error) {
	block := db.ReadBlock(tx, blockHash)

	var root *mpt.MerklePatriciaTrie
	if block != nil {
		root = mpt.NewMerklePatriciaTrieWithRoot(tx, db.ContractTrieTable, block.SmartContractsRoot)
	} else {
		root = mpt.NewMerklePatriciaTrie(tx, db.ContractTrieTable)
	}

	return &ExecutionState{
		Tx:           tx,
		ContractRoot: root,
		PrevBlock:    blockHash,
		Accounts:     map[common.Address]*AccountState{},
	}, nil
}

func (es *ExecutionState) GetAccount(addr common.Address) (*AccountState, error) {
	acc, ok := es.Accounts[addr]
	if ok {
		return acc, nil
	}

	addrHash := addr.Hash()
	accHash, err := es.ContractRoot.Get(addrHash[:])
	if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		return nil, err
	}

	if accHash == nil {
		return nil, nil
	}

	return NewAccountState(es.Tx, common.Hash(accHash))
}

func (as *AccountState) GetState(key common.Hash) (uint256.Int, error) {
	val, ok := as.State[key]
	if ok {
		return val, nil
	}

	rawVal, err := as.StorageRoot.Get(key[:])
	if err != nil {
		return uint256.Int{}, err
	}

	newVal := ssz.UnmarshalUint256SSZ(rawVal)
	as.State[key] = newVal

	return newVal, nil
}

func (as *AccountState) SetBalance(balance uint256.Int) {
	as.Balance = balance
}

func (as *AccountState) SetState(key common.Hash, val uint256.Int) {
	as.State[key] = val
}

func (as *AccountState) Commit() (common.Hash, error) {
	for k, v := range as.State {
		err := as.StorageRoot.Set(k[:], ssz.Uint256SSZ(v))
		if err != nil {
			return common.EmptyHash, err
		}
	}

	acc := types.SmartContract{
		Balance:     as.Balance,
		StorageRoot: as.StorageRoot.RootHash(),
		CodeHash:    as.CodeHash,
	}

	accHash := acc.Hash()

	if err := db.WriteContract(as.Tx, &acc); err != nil {
		return common.EmptyHash, err
	}

	if err := db.WriteCode(as.Tx, as.Code); err != nil {
		return common.EmptyHash, err
	}

	return accHash, nil
}

func (es *ExecutionState) GetState(addr common.Address, key common.Hash) (uint256.Int, error) {
	acc, err := es.GetAccount(addr)
	if err != nil {
		return uint256.Int{}, err
	}
	if acc == nil {
		return uint256.Int{}, nil
	}

	return acc.GetState(key)
}

func (es *ExecutionState) SetState(addr common.Address, key common.Hash, val uint256.Int) error {
	acc, err := es.GetAccount(addr)
	if err != nil {
		return err
	}
	if acc == nil {
		logger.Error().Msgf("failed to find contract while setting state")
		return db.ErrKeyNotFound
	}

	acc.SetState(key, val)
	return nil
}

func (es *ExecutionState) SetBalance(addr common.Address, balance uint256.Int) error {
	acc, err := es.GetAccount(addr)
	if err != nil {
		return err
	}

	acc.SetBalance(balance)
	return nil
}

func (es *ExecutionState) CreateContract(addr common.Address, code types.Code) error {
	acc, err := es.GetAccount(addr)

	if err != nil {
		return err
	}

	if acc != nil {
		return errors.New("contract already exists")
	}

	// TODO: store storage of each contract in separate table
	root := mpt.NewMerklePatriciaTrie(es.Tx, db.StorageTrieTable)

	es.Accounts[addr] = &AccountState{
		Tx:          es.Tx,
		StorageRoot: root,
		CodeHash:    code.Hash(),
		Code:        code,
		State:       map[common.Hash]uint256.Int{},
	}

	return nil
}

func (es *ExecutionState) ContractExists(addr common.Address) (bool, error) {
	acc, err := es.GetAccount(addr)

	return acc != nil, err
}

func (es *ExecutionState) Commit() (common.Hash, error) {
	for k, acc := range es.Accounts {
		v, err := acc.Commit()
		if err != nil {
			return common.EmptyHash, err
		}

		kHash := k.Hash()
		if err = es.ContractRoot.Set(kHash[:], v[:]); err != nil {
			return common.EmptyHash, err
		}
	}

	block := types.Block{
		Id:                 0,
		PrevBlock:          es.PrevBlock,
		SmartContractsRoot: es.ContractRoot.RootHash(),
	}

	blockHash := block.Hash()

	err := db.WriteBlock(es.Tx, &block)
	if err != nil {
		return common.EmptyHash, err
	}

	return blockHash, nil
}
