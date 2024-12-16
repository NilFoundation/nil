package mpttracer

import (
	"errors"

	"github.com/NilFoundation/nil/nil/client/rpc"
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/mpt"
	"github.com/NilFoundation/nil/nil/internal/types"
)

// MPTTracer handles interaction with Merkle Patricia Tries
type MPTTracer struct {
	contractReader     ContractReader
	rwTx               db.RwTx
	shardId            types.ShardId
	accountsCache      map[types.Address]*TracerAccount
	ContractSparseTrie *mpt.MerklePatriciaTrie
}

var _ = (*MPTTracer)(nil)

// New creates a new MPTTracer using a debug API client
func New(
	client *rpc.Client,
	shardBlockNumber types.BlockNumber,
	rwTx db.RwTx,
	shardId types.ShardId,
) *MPTTracer {
	debugApiReader := NewDebugApiContractReader(client, shardBlockNumber, rwTx, shardId)
	return NewWithReader(debugApiReader, rwTx, shardId)
}

// NewWithReader creates a new MPTTracer with a custom contract reader
func NewWithReader(
	contractReader ContractReader,
	rwTx db.RwTx,
	shardId types.ShardId,
) *MPTTracer {
	contractSparseTrie := mpt.NewDbMPT(rwTx, shardId, db.ReceiptTrieTable)
	return &MPTTracer{
		contractReader:     contractReader,
		rwTx:               rwTx,
		shardId:            shardId,
		ContractSparseTrie: contractSparseTrie,
		accountsCache:      make(map[types.Address]*TracerAccount),
	}
}

// CreateAccount creates a new account in the tracer
func (mt *MPTTracer) CreateAccount(addr types.Address) (*TracerAccount, error) {
	existingAccount, err := mt.GetAccount(addr)
	if err != nil {
		return nil, err
	}
	if existingAccount != nil {
		return nil, errors.New("account already exists")
	}

	newAcc, err := NewTracerAccount(mt, addr, nil)
	if err != nil {
		return nil, err
	}

	mt.accountsCache[addr] = newAcc

	return newAcc, nil
}

// GetAccount retrieves an account from the cache or contract reader
func (mt *MPTTracer) GetAccount(addr types.Address) (*TracerAccount, error) {
	// return cached
	smartContract, exists := mt.accountsCache[addr]
	if exists {
		return smartContract, nil
	}

	contract, proof, err := mt.contractReader.GetAccount(addr)
	if err != nil {
		return nil, err
	}

	err = mpt.PopulateMptWithProof(mt.ContractSparseTrie, &proof)
	if err != nil {
		return nil, err
	}

	mt.accountsCache[addr] = contract

	return contract, nil
}

// GetSlot retrieves a slot value for a specific address
func (mt *MPTTracer) GetSlot(addr types.Address, key common.Hash) (common.Hash, error) {
	acc, err := mt.GetAccount(addr)
	if err != nil || acc == nil {
		return common.EmptyHash, err
	}

	return acc.GetState(key)
}

// SetSlot sets a slot value for a specific address
func (mt *MPTTracer) SetSlot(addr types.Address, key common.Hash, val common.Hash) error {
	acc, err := mt.GetAccount(addr)
	if err != nil {
		return err
	}

	err = acc.SetState(key, val)
	if err != nil {
		return err
	}

	return nil
}

// GetAccountsStorageUpdatesTraces retrieves storage update traces for all cached accounts
func (mt *MPTTracer) GetAccountsStorageUpdatesTraces() (map[types.Address][]StorageTrieUpdateTrace, error) {
	storageTracesByAccount := make(map[types.Address][]StorageTrieUpdateTrace)
	for addr, acc := range mt.accountsCache {
		if acc == nil {
			continue
		}
		accTraces, err := acc.GetSlotUpdatesTraces()
		if err != nil {
			return nil, err
		}
		if len(accTraces) != 0 {
			storageTracesByAccount[addr] = accTraces
		}
	}
	return storageTracesByAccount, nil
}

// GetMPTTraces retrieves all MPT traces including storage and contract trie traces
func (mt *MPTTracer) GetMPTTraces() (MPTTraces, error) {
	storageTracesByAccount, err := mt.GetAccountsStorageUpdatesTraces()
	if err != nil {
		return MPTTraces{}, err
	}

	contractTrieTraces, err := mt.GetAccountTrieTraces()
	if err != nil {
		return MPTTraces{}, err
	}
	return MPTTraces{
		StorageTracesByAccount: storageTracesByAccount,
		ContractTrieTraces:     contractTrieTraces,
	}, nil
}

// GetAccountTrieTraces retrieves traces for changes in the contract trie
func (mt *MPTTracer) GetAccountTrieTraces() ([]ContractTrieUpdateTrace, error) {
	contractTrie := execution.NewContractTrie(mt.ContractSparseTrie)
	contractTrieTraces := make([]ContractTrieUpdateTrace, 0, len(mt.accountsCache))
	for addr, acc := range mt.accountsCache {
		if acc == nil {
			continue
		}
		accInTrie, err := contractTrie.Fetch(addr.Hash())
		if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
			return nil, err
		}

		commitedAcc, err := acc.Commit()
		if err != nil {
			return nil, err
		}

		if accInTrie != nil && accInTrie.Hash() == commitedAcc.Hash() {
			continue
		}

		trace := ContractTrieUpdateTrace{
			RootBefore:  contractTrie.RootHash(),
			ValueBefore: accInTrie,
		}
		if err := contractTrie.Update(addr.Hash(), commitedAcc); err != nil {
			return nil, err
		}
		trace.RootAfter = contractTrie.RootHash()
		trace.ValueAfter = commitedAcc

		trace.Proof, err = mpt.BuildProof(contractTrie.Reader, addr.Hash().Bytes(), mpt.SetMPTOperation)
		if err != nil {
			return nil, err
		}
		contractTrieTraces = append(contractTrieTraces, trace)
	}
	return contractTrieTraces, nil
}

// GetRwTx returns the read-write transaction
func (mt *MPTTracer) GetRwTx() db.RwTx {
	return mt.rwTx
}

// AppendToJournal is a no-op method to satisfy the interface
func (mt *MPTTracer) AppendToJournal(je execution.JournalEntry) {}
