package execution

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"

	"github.com/NilFoundation/nil/common"
	db "github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/mpt"
	"github.com/NilFoundation/nil/core/ssz"
	"github.com/NilFoundation/nil/core/tracing"
	"github.com/NilFoundation/nil/core/types"
	"github.com/holiman/uint256"
)

var logger = common.NewLogger("execution", false /* noColor */)

type AccountState struct {
	Tx          db.Tx
	Balance     uint256.Int
	Code        types.Code
	CodeHash    common.Hash
	Seqno       uint64
	StorageRoot *mpt.MerklePatriciaTrie
	ShardId     types.ShardId

	State map[common.Hash]uint256.Int
}

type ExecutionState struct {
	tx               db.Tx
	ContractRoot     *mpt.MerklePatriciaTrie
	MessageRoot      *mpt.MerklePatriciaTrie
	ReceiptRoot      *mpt.MerklePatriciaTrie
	PrevBlock        common.Hash
	MasterChain      common.Hash
	ShardId          types.ShardId
	ChildChainBlocks map[uint64]common.Hash

	Accounts map[common.Address]*AccountState
	Messages []*types.Message
	Receipts []*types.Receipt
}

func (s *AccountState) empty() bool {
	return s.Seqno == 0 && s.Balance.IsZero() && len(s.Code) == 0
}

func NewAccountState(tx db.Tx, shardId types.ShardId, data []byte) (*AccountState, error) {
	account := new(types.SmartContract)

	if err := account.DecodeSSZ(data, 0); err != nil {
		logger.Fatal().Msg("Invalid SSZ while decoding account")
	}

	// TODO: store storage of each contract in separate table
	root := mpt.NewMerklePatriciaTrieWithRoot(tx, db.StorageTrieTableName(shardId), account.StorageRoot)

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
		Seqno:       account.Seqno,
		State:       map[common.Hash]uint256.Int{},
	}, nil
}

func NewExecutionState(tx db.Tx, shardId types.ShardId, blockHash common.Hash) (*ExecutionState, error) {
	block := db.ReadBlock(tx, blockHash)

	var contractRoot, messageRoot, receiptRoot *mpt.MerklePatriciaTrie
	contractTrieTable := db.ContractTrieTableName(shardId)
	messageTrieTable := db.MessageTrieTableName(shardId)
	receiptTrieTable := db.ReceiptTrieTableName(shardId)
	if block != nil {
		contractRoot = mpt.NewMerklePatriciaTrieWithRoot(tx, contractTrieTable, block.SmartContractsRoot)
		messageRoot = mpt.NewMerklePatriciaTrieWithRoot(tx, messageTrieTable, block.MessagesRoot)
		receiptRoot = mpt.NewMerklePatriciaTrieWithRoot(tx, receiptTrieTable, block.ReceiptsRoot)
	} else {
		contractRoot = mpt.NewMerklePatriciaTrie(tx, contractTrieTable)
		messageRoot = mpt.NewMerklePatriciaTrie(tx, messageTrieTable)
		receiptRoot = mpt.NewMerklePatriciaTrie(tx, receiptTrieTable)
	}

	return &ExecutionState{
		tx:               tx,
		ContractRoot:     contractRoot,
		MessageRoot:      messageRoot,
		ReceiptRoot:      receiptRoot,
		PrevBlock:        blockHash,
		ShardId:          shardId,
		ChildChainBlocks: map[uint64]common.Hash{},
		Accounts:         map[common.Address]*AccountState{},
		Messages:         []*types.Message{},
		Receipts:         []*types.Receipt{},
	}, nil
}

func (es *ExecutionState) GetAccount(addr common.Address) *AccountState {
	acc, ok := es.Accounts[addr]
	if ok {
		return acc
	}

	addrHash := addr.Hash()

	data, err := es.ContractRoot.Get(addrHash[:])
	if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		panic(fmt.Sprintf("failed to fetch account %v: %v", addrHash, err))
	}

	if data == nil {
		return nil
	}

	acc, err = NewAccountState(es.tx, es.ShardId, data)
	if err != nil {
		panic(fmt.Sprintf("failed to create account on shard %v: %v", es.ShardId, err))
	}
	es.Accounts[addr] = acc
	return acc
}

func (es *ExecutionState) AddAddressToAccessList(addr common.Address) {
}

func (es *ExecutionState) AddBalance(addr common.Address, amount *uint256.Int, reason tracing.BalanceChangeReason) {
	acc := es.getOrNewAccount(addr)
	acc.Balance.Add(&acc.Balance, amount)
}

func (es *ExecutionState) AddLog(*types.Log) {
	panic("unimplemented")
}

func (es *ExecutionState) AddRefund(uint64) {
	panic("unimplemented")
}

func (es *ExecutionState) AddSlotToAccessList(addr common.Address, slot common.Hash) {
}

func (es *ExecutionState) AddressInAccessList(addr common.Address) bool {
	return true // FIXME
}

func (es *ExecutionState) Empty(addr common.Address) bool {
	acc := es.GetAccount(addr)
	return acc == nil || acc.empty()
}

func (es *ExecutionState) Exist(addr common.Address) bool {
	acc := es.GetAccount(addr)
	return acc != nil
}

func (es *ExecutionState) GetCode(addr common.Address) []byte {
	acc := es.GetAccount(addr)
	if acc != nil {
		return acc.Code
	}
	return nil
}

func (es *ExecutionState) GetCodeHash(addr common.Address) common.Hash {
	acc := es.GetAccount(addr)
	if acc != nil {
		return acc.CodeHash
	}
	return common.EmptyHash
}

func (es *ExecutionState) GetCodeSize(addr common.Address) int {
	acc := es.GetAccount(addr)
	if acc != nil {
		return len(acc.Code)
	}
	return 0
}

func (es *ExecutionState) GetCommittedState(common.Address, common.Hash) common.Hash {
	return common.EmptyHash
}

func (es *ExecutionState) GetRefund() uint64 {
	panic("unimplemented")
}

func (es *ExecutionState) GetStorageRoot(addr common.Address) common.Hash {
	acc := es.GetAccount(addr)
	if acc != nil {
		return acc.StorageRoot.RootHash()
	}
	return common.EmptyHash
}

func (es *ExecutionState) GetTransientState(addr common.Address, key common.Hash) common.Hash {
	panic("unimplemented")
}

func (es *ExecutionState) HasSelfDestructed(common.Address) bool {
	panic("unimplemented")
}

func (es *ExecutionState) Selfdestruct6780(common.Address) {
	panic("unimplemented")
}

func (es *ExecutionState) SetCode(addr common.Address, code []byte) {
	acc := es.GetAccount(addr)
	acc.Code = code
}

func (es *ExecutionState) SetTransientState(addr common.Address, key common.Hash, value common.Hash) {
	panic("unimplemented")
}

func (es *ExecutionState) SlotInAccessList(addr common.Address, slot common.Hash) (addressOk bool, slotOk bool) {
	return true, true // FIXME
}

func (es *ExecutionState) SubBalance(common.Address, *uint256.Int, tracing.BalanceChangeReason) {
	panic("unimplemented")
}

func (es *ExecutionState) SubRefund(uint64) {
	panic("unimplemented")
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

func (as *AccountState) SetSeqno(seqno uint64) {
	as.Seqno = seqno
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
		Seqno:       as.Seqno,
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
	acc := es.GetAccount(addr)
	if acc == nil {
		return common.EmptyHash
	}

	value, err := acc.GetState(key)
	if err != nil {
		panic(err)
	}
	return value.Bytes32()
}

func (es *ExecutionState) SetState(addr common.Address, key common.Hash, val common.Hash) {
	acc := es.GetAccount(addr)
	if acc == nil {
		panic(fmt.Sprintf("failed to find contract %v", addr))
	}

	acc.SetState(key, *val.Uint256())
}

func (es *ExecutionState) GetBalance(addr common.Address) *uint256.Int {
	acc := es.GetAccount(addr)
	if acc == nil {
		return uint256.NewInt(0)
	}
	return &acc.Balance
}

func (es *ExecutionState) GetSeqno(addr common.Address) uint64 {
	acc := es.GetAccount(addr)
	if acc == nil {
		return 0
	}
	return acc.Seqno
}

func (s *ExecutionState) getOrNewAccount(addr common.Address) *AccountState {
	acc := s.GetAccount(addr)
	if acc != nil {
		return acc
	}
	err := s.CreateContract(addr, nil)
	if err != nil {
		panic(err)
	}
	return s.GetAccount(addr)
}

func (es *ExecutionState) SetBalance(addr common.Address, balance uint256.Int) {
	acc := es.getOrNewAccount(addr)
	acc.SetBalance(balance)
}

func (es *ExecutionState) SetSeqno(addr common.Address, seqno uint64) {
	acc := es.getOrNewAccount(addr)
	acc.SetSeqno(seqno)
}

func (es *ExecutionState) SetMasterchainHash(masterChainHash common.Hash) {
	es.MasterChain = masterChainHash
}

func (es *ExecutionState) SetShardHash(shardId uint64, hash common.Hash) {
	es.ChildChainBlocks[shardId] = hash
}

func (es *ExecutionState) CreateContract(addr common.Address, code types.Code) error {
	acc := es.GetAccount(addr)

	if acc != nil {
		return errors.New("contract already exists")
	}

	// TODO: store storage of each contract in separate table
	root := mpt.NewMerklePatriciaTrie(es.tx, db.StorageTrieTableName(es.ShardId))

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

func (es *ExecutionState) ContractExists(addr common.Address) bool {
	acc := es.GetAccount(addr)
	return acc != nil
}

// CreateAddress creates an ethereum address given the bytes and the nonce
func CreateAddress(b common.Address, nonce uint64) common.Address {
	data, err := ssz.MarshalSSZ(nil, b[:], nonce)
	if err != nil {
		logger.Fatal().Err(err).Msgf("MarshalSSZ failed on: %v, %v", b, nonce)
	}
	return common.BytesToAddress(data)
}

func (es *ExecutionState) AddMessage(message *types.Message) {
	message.Index = uint64(len(es.Messages))
	es.Messages = append(es.Messages, message)

	// Deploy message
	if bytes.Equal(message.To[:], common.EmptyAddress[:]) {
		addr := CreateAddress(message.From, message.Seqno)

		var r types.Receipt
		r.Success = true
		r.ContractAddress = addr
		r.MsgHash = message.Hash()
		r.MsgIndex = message.Index

		// TODO: gasUsed
		if err := es.CreateContract(addr, message.Data); err != nil {
			logger.Fatal().Err(err).Msgf("Failed to create contract")
		}

		es.Receipts = append(es.Receipts, &r)
	}
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
		treeShards := mpt.NewMerklePatriciaTrie(es.tx, db.ShardBlocksTrieTableName(blockId))
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
		k, err := ssz.MarshalSSZ(nil, m.Index)
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

	for _, r := range es.Receipts {
		r.BlockNumber = blockId
		v, err := r.MarshalSSZ()
		if err != nil {
			return common.EmptyHash, err
		}
		k, err := ssz.MarshalSSZ(nil, r.MsgIndex)
		if err != nil {
			return common.EmptyHash, err
		}
		if err := es.ReceiptRoot.Set(k, v); err != nil {
			return common.EmptyHash, err
		}
	}

	block := types.Block{
		Id:                  blockId,
		PrevBlock:           es.PrevBlock,
		SmartContractsRoot:  es.ContractRoot.RootHash(),
		MessagesRoot:        es.MessageRoot.RootHash(),
		ReceiptsRoot:        es.ReceiptRoot.RootHash(),
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
