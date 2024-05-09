package execution

import (
	"errors"
	"fmt"
	"strconv"

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
	ShardId     int

	State map[common.Hash]uint256.Int
}

type ExecutionState struct {
	Tx           db.Tx
	ContractRoot *mpt.MerklePatriciaTrie
	PrevBlock    common.Hash
	ShardId      int

	Accounts map[common.Address]*AccountState
}

func NewAccountState(tx db.Tx, shardId int, data []byte) (*AccountState, error) {
	account := new(types.SmartContract)

	if err := account.DecodeSSZ(data, 0); err != nil {
		logger.Fatal().Msg("Invalid SSZ while decoding account")
	}

	// TODO: store storage of each contract in separate table
	root := mpt.NewMerklePatriciaTrieWithRoot(tx, db.TableName(db.StorageTrieTable, shardId), account.StorageRoot)

	code, err := db.ReadCode(tx, shardId, account.CodeHash)

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
		ShardId:     shardId,
		State:       map[common.Hash]uint256.Int{},
	}, nil
}

func NewExecutionState(tx db.Tx, shardId int, blockHash common.Hash) (*ExecutionState, error) {
	block := db.ReadBlock(tx, blockHash)

	var root *mpt.MerklePatriciaTrie
	if block != nil {
		root = mpt.NewMerklePatriciaTrieWithRoot(tx, db.TableName(db.ContractTrieTable, shardId), block.SmartContractsRoot)
	} else {
		root = mpt.NewMerklePatriciaTrie(tx, db.TableName(db.ContractTrieTable, shardId))
	}

	return &ExecutionState{
		RoTx:         roTx,
		RwTx:         rwTx,
		ContractRoot: root,
		PrevBlock:    blockHash,
		ShardId:      shardId,
		Accounts:     map[common.Address]*AccountState{},
	}, nil
}

func (es *ExecutionState) GetAccount(addr common.Address) (*AccountState, error) {
	acc, ok := es.Accounts[addr]
	if ok {
		return acc, nil
	}

	addrHash := addr.Hash()

	data, err := es.ContractRoot.Get(addrHash[:])
	if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		return nil, err
	}

	if data == nil {
		return nil, nil
	}

	acc, err = NewAccountState(es.Tx, es.ShardId, data)
	if err != nil {
		return nil, err
	}
	es.Accounts[addr] = acc
	return acc, nil
}

func (as *AccountState) GetState(key common.Hash) (uint256.Int, error) {
	val, ok := as.State[key]
	if ok {
		return val, nil
	}

	rawVal, err := as.StorageRoot.Get(key[:])
	if err == db.ErrKeyNotFound {
		return uint256.Int{}, nil
	}
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

func (as *AccountState) Commit() ([]byte, error) {
	for k, v := range as.State {
		err := as.StorageRoot.Set(k[:], ssz.Uint256SSZ(v))
		if err != nil {
			return nil, err
		}
	}

	acc := types.SmartContract{
		Balance:     as.Balance,
		StorageRoot: as.StorageRoot.RootHash(),
		CodeHash:    as.CodeHash,
	}

	data, err := acc.EncodeSSZ(nil)
	if err != nil {
		return nil, err
	}

	if err := db.WriteCode(as.Tx, as.ShardId, as.Code); err != nil {
		return nil, err
	}

	return data, nil
}

func (es *ExecutionState) GetState(addr common.Address, key common.Hash) common.Hash {
	acc, err := es.GetAccount(addr)
	if err != nil {
		panic(err)
	}
	if acc == nil {
		return common.EmptyHash
	}

	value, err := acc.GetState(key)
	if err != nil {
		panic(err)
	}
	return value.Bytes32()
}

func (es *ExecutionState) SetState(addr common.Address, key common.Hash, val common.Hash) error {
	acc, err := es.GetAccount(addr)
	if err != nil {
		return err
	}
	if acc == nil {
		logger.Error().Msgf("failed to find contract while setting state")
		return db.ErrKeyNotFound
	}

	acc.SetState(key, *val.Uint256())
	return nil
}

func (es *ExecutionState) GetBalance(addr common.Address) uint256.Int {
	acc, err := es.GetAccount(addr)
	if err != nil || acc == nil {
		return uint256.Int{}
	}
	return acc.Balance
}

func (es *ExecutionState) SetBalance(addr common.Address, balance uint256.Int) error {
	acc, err := es.GetAccount(addr)
	if err != nil {
		return err
	}

	acc.SetBalance(balance)
	return nil
}

func (es *ExecutionState) SetMasterchainHash() error {
	masterchainBlockRaw, err := es.RoTx.Get(db.LastBlockTable, []byte(strconv.Itoa(0)))
	if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		return fmt.Errorf("failed getting last masterchain block %w", err)
	}
	if masterchainBlockRaw != nil {
		es.MasterChain = common.Hash(*masterchainBlockRaw)
	}
	return nil
}

func (es *ExecutionState) SetShardHash(nshards int, prevChildBlocksRootHash common.Hash) error {
	treeShards := mpt.NewMerklePatriciaTrieWithRoot(es.RwTx, db.ShardsBlocksTrieTable, prevChildBlocksRootHash)
	for i := 1; i < nshards; i++ {
		lastBlockRaw, err := es.RoTx.Get(db.LastBlockTable, []byte(strconv.Itoa(i)))
		if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
			return fmt.Errorf("failed getting last block %w for shard %d", err, i)
		}
		lastBlockHash := common.EmptyHash
		if lastBlockRaw != nil {
			lastBlockHash = common.Hash(*lastBlockRaw)
		}
		if err := treeShards.Set([]byte(strconv.Itoa(i)), []byte(lastBlockHash.String())); err != nil {
			return err
		}
	}
	es.ChildBlocksRootHash = treeShards.RootHash()
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
	root := mpt.NewMerklePatriciaTrie(es.Tx, db.TableName(db.StorageTrieTable, es.ShardId))

	es.Accounts[addr] = &AccountState{
		Tx:          es.RwTx,
		StorageRoot: root,
		CodeHash:    code.Hash(),
		Code:        code,
		ShardId:     es.ShardId,
		State:       map[common.Hash]uint256.Int{},
	}

	return nil
}

func (es *ExecutionState) ContractExists(addr common.Address) (bool, error) {
	acc, err := es.GetAccount(addr)

	return acc != nil, err
}

func (es *ExecutionState) Commit(isMasterchain bool, nshards int) (common.Hash, error) {
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

	if isMasterchain {
		prevChildRootHash := common.EmptyHash
		if es.PrevBlock != common.EmptyHash {
			prevChildRootHash = db.ReadBlock(es.RwTx, es.PrevBlock).ChildBlocksRootHash
		}
		if err := es.SetShardHash(nshards, prevChildRootHash); err != nil {
			return common.EmptyHash, err
		}
	} else {
		if err := es.SetMasterchainHash(); err != nil {
			return common.EmptyHash, err
		}
	}

	blockId := uint64(0)
	if es.PrevBlock != common.EmptyHash {
		blockId = db.ReadBlock(es.RwTx, es.PrevBlock).Id + 1
	}

	block := types.Block{
		Id:                  blockId,
		PrevBlock:           es.PrevBlock,
		SmartContractsRoot:  es.ContractRoot.RootHash(),
		ChildBlocksRootHash: es.ChildBlocksRootHash,
		MasterChainHash:     es.MasterChain,
	}

	blockHash := block.Hash()

	err := db.WriteBlock(es.RwTx, &block)
	if err != nil {
		return common.EmptyHash, err
	}

	return blockHash, nil
}
