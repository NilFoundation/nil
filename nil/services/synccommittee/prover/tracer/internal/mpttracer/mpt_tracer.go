package mpttracer

import (
	"context"
	"errors"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/mpt"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/synccommittee/prover/tracer/api"
)

// MPTTracer handles interaction with Merkle Patricia Tries
type MPTTracer struct {
	// es *execution.ExecutionState,
	contractReader      ContractReader
	rwTx                db.RwTx
	shardId             types.ShardId
	ContractSparseTrie  *mpt.MerklePatriciaTrie
	initialContractRoot common.Hash
	// since we can't iterate over sparse trie, keep accounts for explicit checks
	touchedAccounts map[types.Address]struct{}
}

var _ execution.IContractMPTRepository = (*MPTTracer)(nil)

func (mt *MPTTracer) SetRootHash(root common.Hash) {
	mt.ContractSparseTrie.SetRootHash(root)
	if mt.initialContractRoot == common.EmptyHash {
		// first call, save this root to compare state with it during traces generation
		mt.initialContractRoot = root
	}
}

func (mt *MPTTracer) GetContract(addr types.Address) (*types.SmartContract, error) {
	contractTrie := execution.NewContractTrie(mt.ContractSparseTrie)

	// try to fetch from cache
	smartContract, err := contractTrie.Fetch(addr.Hash())
	if smartContract != nil {
		// we fetched this contract before (in could be even updated by this time)
		return smartContract, nil
	}
	if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		return nil, err
	}

	// TODO: use meaningful context
	contract, proof, err := mt.contractReader.GetAccount(context.Background(), addr)
	if err != nil {
		return nil, err
	}

	rootBeforePupulation := mt.ContractSparseTrie.RootHash()
	err = mpt.PopulateMptWithProof(mt.ContractSparseTrie, &proof)
	if err != nil {
		return nil, err
	}
	mt.ContractSparseTrie.SetRootHash(rootBeforePupulation)

	if contract == nil {
		err = db.ErrKeyNotFound
	}
	return contract, err
}

func (mt *MPTTracer) UpdateContracts(contracts map[types.Address]*execution.AccountState) error {
	keys := make([]common.Hash, 0, len(contracts))
	values := make([]*types.SmartContract, 0, len(contracts))
	for addr, acc := range contracts {
		mt.touchedAccounts[addr] = struct{}{}

		smartAccount, err := acc.Commit()
		if err != nil {
			return err
		}
		keys = append(keys, addr.Hash())
		values = append(values, smartAccount)
	}
	contractTrie := execution.NewContractTrie(mt.ContractSparseTrie)

	err := contractTrie.UpdateBatch(keys, values)
	return err
}

func (mt *MPTTracer) RootHash() common.Hash {
	return mt.ContractSparseTrie.RootHash()
}

// New creates a new MPTTracer using a debug API client
func New(
	client api.RpcClient,
	shardBlockNumber types.BlockNumber,
	rwTx db.RwTx,
	shardId types.ShardId,
) *MPTTracer {
	debugApiReader := NewDebugApiContractReader(client, shardBlockNumber, rwTx, shardId)
	return NewWithReader(debugApiReader, rwTx, shardId)
}

// NewWithReader creates a new MPTTracer with a provided contract reader
func NewWithReader(
	contractReader ContractReader,
	rwTx db.RwTx,
	shardId types.ShardId,
) *MPTTracer {
	contractSparseTrie := mpt.NewDbMPT(rwTx, shardId, db.ConfigTrieTable)
	return &MPTTracer{
		contractReader:     contractReader,
		rwTx:               rwTx,
		shardId:            shardId,
		ContractSparseTrie: contractSparseTrie,
		touchedAccounts:    make(map[types.Address]struct{}),
	}
}

// GetMPTTraces retrieves all MPT traces including storage and contract trie traces
func (mt *MPTTracer) GetMPTTraces() (MPTTraces, error) {
	// TODO: in case of node deletion from MPT (SELFDESTRUCT and zero balance),
	// extra nodes (not fetched previously) could be required, currently this is not handled.
	contractTrie := execution.NewContractTrie(mt.ContractSparseTrie)
	curRoot := mt.ContractSparseTrie.RootHash()

	storageTracesByAccount := make(map[types.Address][]StorageTrieUpdateTrace)
	for addr := range mt.touchedAccounts {
		// contractTrie.SetRootHash() affects underlying ContractSparseTrie, so we need to change
		// it each time we switch between current and initial tries
		contractTrie.SetRootHash(mt.initialContractRoot)
		initialSmartContract, err := contractTrie.Fetch(addr.Hash())
		if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
			return MPTTraces{}, err
		}

		contractTrie.SetRootHash(curRoot)
		currentSmartContract, err := contractTrie.Fetch(addr.Hash())
		if err != nil {
			return MPTTraces{}, err
		}

		if initialSmartContract == nil || initialSmartContract.StorageRoot != currentSmartContract.StorageRoot {
			initialRootToCompare := common.EmptyHash
			if initialSmartContract != nil {
				initialRootToCompare = initialSmartContract.StorageRoot
			}
			storageTraces, err := mt.getStorageTraces(initialRootToCompare, currentSmartContract.StorageRoot)
			if err != nil {
				return MPTTraces{}, err
			}
			storageTracesByAccount[addr] = storageTraces
		}
	}

	contractTrieTraces, err := mt.getAccountTrieTraces(mt.initialContractRoot, curRoot)
	if err != nil {
		return MPTTraces{}, err
	}
	return MPTTraces{
		StorageTracesByAccount: storageTracesByAccount,
		ContractTrieTraces:     contractTrieTraces,
	}, nil
}

func getTrieTraces[V any, VPtr execution.MPTValue[V]](
	rawTrie *mpt.MerklePatriciaTrie,
	trieCtor func(parent *mpt.MerklePatriciaTrie) *execution.BaseMPT[common.Hash, V, VPtr],
	initialRoot common.Hash,
	currentRoot common.Hash,
) ([]GenericTrieUpdateTrace[VPtr], error) {
	trie := trieCtor(rawTrie)

	trie.SetRootHash(initialRoot)
	initialEntries, err := trie.Entries()
	if err != nil {
		return nil, err
	}
	initialEntriesMap := make(map[common.Hash]VPtr, len(initialEntries))
	for _, e := range initialEntries {
		initialEntriesMap[e.Key] = e.Val
	}

	trie.SetRootHash(currentRoot)
	currentEntries, err := trie.Entries()
	if err != nil {
		return nil, err
	}

	traces := make([]GenericTrieUpdateTrace[VPtr], 0, len(currentEntries)) // can't establish final size here
	trie.SetRootHash(initialRoot)
	for _, e := range currentEntries {
		initialValue, exists := initialEntriesMap[e.Key]
		if exists {
			// delete from initial entries map, every key left in map was deleted within execution
			delete(initialEntriesMap, e.Key)

			if initialValue == e.Val {
				// value was not changed, no trace required
				continue
			}
		}

		// ReadMPTOperation plays no role here, could be any
		proof, err := mpt.BuildProof(trie.Reader, e.Key.Bytes(), mpt.ReadMPTOperation)
		if err != nil {
			return nil, err
		}

		slotChangeTrace := GenericTrieUpdateTrace[VPtr]{
			Key:        e.Key,
			RootBefore: trie.RootHash(),
			PathBefore: proof.PathToNode,
			ValueAfter: e.Val,
			Proof:      proof,
		}

		if exists && initialValue != e.Val {
			slotChangeTrace.ValueBefore = initialValue
			// update happened
		} // else insertion happened

		if err := trie.Update(e.Key, e.Val); err != nil {
			return nil, err
		}

		// ReadMPTOperation plays no role here, could be any
		proof, err = mpt.BuildProof(trie.Reader, e.Key.Bytes(), mpt.ReadMPTOperation)
		if err != nil {
			return nil, err
		}

		slotChangeTrace.RootAfter = trie.RootHash()
		slotChangeTrace.PathAfter = proof.PathToNode

		traces = append(traces, slotChangeTrace)
	}
	for k, v := range initialEntriesMap {
		// deletion happened

		// ReadMPTOperation plays no role here, could be any
		proof, err := mpt.BuildProof(trie.Reader, k.Bytes(), mpt.ReadMPTOperation)
		if err != nil {
			return nil, err
		}

		slotChangeTrace := GenericTrieUpdateTrace[VPtr]{
			Key:         k,
			RootBefore:  trie.RootHash(),
			PathBefore:  proof.PathToNode,
			ValueBefore: v,
			Proof:       proof,
		}

		if err := trie.Delete(k); err != nil {
			return nil, err
		}

		// ReadMPTOperation plays no role here, could be any
		proof, err = mpt.BuildProof(trie.Reader, k.Bytes(), mpt.ReadMPTOperation)
		if err != nil {
			return nil, err
		}

		slotChangeTrace.RootAfter = trie.RootHash()
		slotChangeTrace.PathAfter = proof.PathToNode
		traces = append(traces, slotChangeTrace)
	}

	return traces, nil
}

// getAccountTrieTraces retrieves traces for changes in a storage trie.
func (mt *MPTTracer) getStorageTraces(
	initialRoot common.Hash,
	currentRoot common.Hash,
) ([]StorageTrieUpdateTrace, error) {
	rawMpt := mpt.NewDbMPT(mt.rwTx, mt.shardId, db.StorageTrieTable)
	return getTrieTraces(rawMpt, execution.NewStorageTrie, initialRoot, currentRoot)
}

// getAccountTrieTraces retrieves traces for changes in a contract trie.
func (mt *MPTTracer) getAccountTrieTraces(
	initialRoot common.Hash, currentRoot common.Hash,
) ([]ContractTrieUpdateTrace, error) {
	rawMpt := mpt.NewDbMPT(mt.rwTx, mt.shardId, db.ConfigTrieTable)
	return getTrieTraces(rawMpt, execution.NewContractTrie, initialRoot, currentRoot)
}
