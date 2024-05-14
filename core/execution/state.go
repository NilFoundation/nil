package execution

import (
	"errors"
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
	tx               db.Tx
	ContractRoot     *mpt.MerklePatriciaTrie
	MessageRoot      *mpt.MerklePatriciaTrie
	PrevBlock        common.Hash
	MasterChain      common.Hash
	ShardId          int
	ChildChainBlocks map[uint64]common.Hash

	Accounts map[common.Address]*AccountState
	Messages []*types.Message
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

	var contractRoot, messageRoot *mpt.MerklePatriciaTrie
	if block != nil {
		contractRoot = mpt.NewMerklePatriciaTrieWithRoot(tx, db.ContractTrieTable, block.SmartContractsRoot)
		messageRoot = mpt.NewMerklePatriciaTrieWithRoot(tx, db.MessageTrieTable, block.MessagesRoot)
	} else {
		contractRoot = mpt.NewMerklePatriciaTrie(tx, db.ContractTrieTable)
		messageRoot = mpt.NewMerklePatriciaTrie(tx, db.MessageTrieTable)
	}

	return &ExecutionState{
		tx:               tx,
		ContractRoot:     contractRoot,
		MessageRoot:      messageRoot,
		PrevBlock:        blockHash,
		ShardId:          shardId,
		ChildChainBlocks: map[uint64]common.Hash{},
		Accounts:         map[common.Address]*AccountState{},
		Messages:         []*types.Message{},
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

	acc, err = NewAccountState(es.tx, es.ShardId, data)
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

func (es *ExecutionState) SetMasterchainHash(masterChainHash common.Hash) {
	es.MasterChain = masterChainHash
}

func (es *ExecutionState) SetShardHash(shardId uint64, hash common.Hash) {
	es.ChildChainBlocks[shardId] = hash
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
	root := mpt.NewMerklePatriciaTrie(es.tx, db.TableName(db.StorageTrieTable, es.ShardId))

	es.Accounts[addr] = &AccountState{
		Tx:          es.tx,
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

func (es *ExecutionState) AddMessage(message *types.Message) {
	message.Index = uint64(len(es.Messages))
	es.Messages = append(es.Messages, message)
}

func (es *ExecutionState) Commit(blockId uint64) (common.Hash, error) {
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

	treeShardsRootHash := common.EmptyHash
	if len(es.ChildChainBlocks) > 0 {
		treeShards := mpt.NewMerklePatriciaTrie(es.tx, db.ShardBlocksTrieTable+strconv.FormatUint(blockId, 10))
		for k, hash := range es.ChildChainBlocks {
			key := strconv.AppendUint(nil, k, 10)
			if err := treeShards.Set(key, hash.Bytes()); err != nil {
				return common.EmptyHash, err
			}
		}
		treeShardsRootHash = treeShards.RootHash()
	}

	for _, m := range es.Messages {
		v, err := m.EncodeSSZ(nil)
		if err != nil {
			return common.EmptyHash, err
		}
		messageId := types.MessageId{BlockId: blockId, MessageIndex: m.Index}
		k, err := messageId.EncodeSSZ(nil)
		if err != nil {
			return common.EmptyHash, err
		}
		if err := es.MessageRoot.Set(k, v); err != nil {
			return common.EmptyHash, err
		}
		if err := db.WriteMessage(es.tx, es.ShardId, m); err != nil {
			return common.EmptyHash, err
		}
	}

	block := types.Block{
		Id:                  blockId,
		PrevBlock:           es.PrevBlock,
		SmartContractsRoot:  es.ContractRoot.RootHash(),
		ChildBlocksRootHash: treeShardsRootHash,
		MasterChainHash:     es.MasterChain,
	}

	blockHash := block.Hash()

	err := db.WriteBlock(es.tx, &block)
	if err != nil {
		return common.EmptyHash, err
	}

	return blockHash, nil
}
