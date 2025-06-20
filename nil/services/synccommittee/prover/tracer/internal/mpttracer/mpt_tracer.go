package mpttracer

import (
	"context"
	"errors"

	"github.com/NilFoundation/nil/nil/client"
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/mpt"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/transport"
	"github.com/ethereum/go-ethereum/common/hexutil"
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
	// in addition, not every touched acc would be committed into state
	updatedAccounts map[types.Address]struct{}

	accountsTraceableStates map[types.Address]*TraceableAccount

	client      client.Client
	blockNumber types.BlockNumber
	logger      logging.Logger
}

var (
	_ execution.IContractMPTRepository = (*MPTTracer)(nil)
	_ execution.DbRwTxProvider         = (*MPTTracer)(nil)
)

func (mt *MPTTracer) SetRootHash(root common.Hash) error {
	if mt.initialContractRoot == common.EmptyHash {
		// first call, save this root to compare state with it during traces generation
		mt.initialContractRoot = root
		return mt.ContractSparseTrie.SetRootHash(mpt.EmptyRootHash)
	}
	return mt.ContractSparseTrie.SetRootHash(root)
}

func (mt *MPTTracer) GetAccountState(addr types.Address, createIfNotExists bool) (execution.AccountState, error) {
	contractTrie := execution.NewContractTrie(mt.ContractSparseTrie)

	// try to fetch from cache
	smartContract, err := contractTrie.Fetch(addr.Hash())
	if smartContract != nil {
		// we fetched this contract before (in could be even updated by this time)
		return mt.accountsTraceableStates[addr], nil
	}
	if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		return nil, err
	}

	// TODO: use meaningful context
	contract, proof, err := mt.contractReader.GetAccount(context.Background(), addr)
	if err != nil {
		return nil, err
	}

	rootBeforePopulation, err := mt.ContractSparseTrie.Commit()
	if err != nil {
		return nil, err
	}
	if rootBeforePopulation == mpt.EmptyRootHash {
		rootBeforePopulation = mt.initialContractRoot
	}
	if err := mpt.PopulateMptWithProof(mt.ContractSparseTrie, &proof); err != nil {
		return nil, err
	}
	if err := mt.ContractSparseTrie.SetRootHash(rootBeforePopulation); err != nil {
		return nil, err
	}

	if contract == nil && !createIfNotExists {
		return nil, nil
	}

	traceableAcc, err := NewTracableAccountState(mt, addr, contract, mt.logger)
	if err != nil {
		return nil, err
	}
	mt.accountsTraceableStates[addr] = traceableAcc
	return traceableAcc, nil
}

func (mt *MPTTracer) UpdateContracts(contracts map[types.Address]execution.AccountState) error {
	keys := make([]common.Hash, 0, len(contracts))
	values := make([]*types.SmartContract, 0, len(contracts))
	for addr, acc := range contracts {
		mt.updatedAccounts[addr] = struct{}{}

		smartAccount, err := acc.Commit()
		if err != nil {
			return err
		}
		keys = append(keys, addr.Hash())
		values = append(values, smartAccount)
	}
	contractTrie := execution.NewContractTrie(mt.ContractSparseTrie)

	if err := contractTrie.UpdateBatch(keys, values); err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			// TODO: if `db.KeyNotFound`, fetch it with `debug_accountRange`
			panic("not implemented")
		}
		return err
	}

	return nil
}

func (mt *MPTTracer) RootHash() common.Hash {
	return mt.ContractSparseTrie.RootHash()
}

func (mt *MPTTracer) Commit() (common.Hash, error) {
	if len(mt.updatedAccounts) == 0 {
		return mt.initialContractRoot, nil
	}
	return mt.ContractSparseTrie.Commit()
}

// New creates a new MPTTracer using a debug API client
func New(
	client client.Client,
	shardBlockNumber types.BlockNumber,
	rwTx db.RwTx,
	shardId types.ShardId,
	logger logging.Logger,
) *MPTTracer {
	debugApiReader := NewDebugApiContractReader(client, shardBlockNumber, rwTx, shardId, logger)
	return NewWithReader(debugApiReader, rwTx, shardId, shardBlockNumber, client, logger)
}

// NewWithReader creates a new MPTTracer with a provided contract reader
func NewWithReader(
	contractReader ContractReader,
	rwTx db.RwTx,
	shardId types.ShardId,
	blockNumber types.BlockNumber,
	client client.Client,
	logger logging.Logger,
) *MPTTracer {
	contractSparseTrie := mpt.NewDbMPT(rwTx, shardId, db.ContractTrieTable)
	check.PanicIfErr(contractSparseTrie.SetRootHash(mpt.EmptyRootHash))
	return &MPTTracer{
		contractReader:          contractReader,
		rwTx:                    rwTx,
		shardId:                 shardId,
		ContractSparseTrie:      contractSparseTrie,
		updatedAccounts:         make(map[types.Address]struct{}),
		accountsTraceableStates: make(map[types.Address]*TraceableAccount),
		client:                  client,
		blockNumber:             blockNumber,
		logger:                  logger,
	}
}

func (mt *MPTTracer) GetZethCache(ctx context.Context) (*FileProviderCache, error) {
	mt.logger.Debug().Msg("getting zeth cache")

	getProofCache := make([]GetProofCache, 0, len(mt.accountsTraceableStates)*2)
	transactionCountCache := make([]TransactionCountCache, 0, len(mt.accountsTraceableStates))
	balanceCache := make([]BalanceCache, 0, len(mt.accountsTraceableStates))
	codeCache := make([]CodeCache, 0, len(mt.accountsTraceableStates))
	storageCache := make([]StorageCache, 0, len(mt.accountsTraceableStates))

	if mt.blockNumber == 0 {
		return nil, errors.New("genesis block cache not supported; block number must be > 0")
	}
	blockNumToFetch := transport.BlockNumber(mt.blockNumber)
	for addr, accountState := range mt.accountsTraceableStates {
		keysToProve := make(map[common.Hash]struct{}, len(accountState.initialSlots)+len(accountState.slotsUpdates))
		for key, value := range accountState.initialSlots {
			storageCache = append(storageCache, StorageCache{
				Args: StorageArgs{
					BlockArgs: BlockArgs{
						BlockNo: blockNumToFetch.Uint64(),
						ShardID: uint64(mt.shardId),
					},
					Address: addr,
					Key:     hexutil.U256(*key.Uint256()),
				},
				Storage: hexutil.U256(*value.Uint256()),
			})
			keysToProve[key] = struct{}{}
		}

		// initial proofs
		keysArg := make([]common.Hash, 0, len(keysToProve))
		for k := range keysToProve {
			keysArg = append(keysArg, k)
		}
		proof, err := mt.client.GetProof(ctx, addr, keysArg, blockNumToFetch)
		if err != nil {
			return nil, err
		}
		getProofCache = append(getProofCache, GetProofCache{
			Args: GetProofArgs{
				BlockNo: blockNumToFetch.Uint64(),
				Address: addr,
				Indices: keysArg,
			},
			Proof: *proof,
		})

		transactionCountCache = append(transactionCountCache, TransactionCountCache{
			Args: TransactionCountArgs{
				BlockArgs: BlockArgs{
					BlockNo: blockNumToFetch.Uint64(),
					ShardID: uint64(mt.shardId),
				},
				Address: addr,
			},
			Seqno: accountState.initialValues.Seqno,
		})
		balanceCache = append(balanceCache, BalanceCache{
			Args: BalanceArgs{
				BlockArgs: BlockArgs{
					BlockNo: blockNumToFetch.Uint64(),
					ShardID: uint64(mt.shardId),
				},
				Address: addr,
			},
			Balance: accountState.initialValues.Balance,
		})

		code, err := db.ReadCode(mt.rwTx, mt.shardId, accountState.initialValues.CodeHash)
		if err != nil {
			return nil, err
		}
		codeCache = append(codeCache, CodeCache{
			Args: CodeArgs{
				BlockArgs: BlockArgs{
					BlockNo: blockNumToFetch.Uint64(),
					ShardID: uint64(mt.shardId),
				},
				Address: addr,
			},
			Code: code,
		})

		// latest proofs (initial keys + updated ones for N+1 block)
		for key := range accountState.slotsUpdates {
			keysToProve[key] = struct{}{}
		}
		keysArg = make([]common.Hash, 0, len(keysToProve))
		for k := range keysToProve {
			keysArg = append(keysArg, k)
		}
		proof, err = mt.client.GetProof(ctx, addr, keysArg, transport.BlockNumber(mt.blockNumber+1))
		if err != nil {
			return nil, err
		}
		getProofCache = append(getProofCache, GetProofCache{
			Args: GetProofArgs{
				BlockNo: mt.blockNumber.Uint64(),
				Address: addr,
				Indices: keysArg,
			},
			Proof: *proof,
		})
	}

	return &FileProviderCache{
		Proofs:           getProofCache,
		TransactionCount: transactionCountCache,
		Balance:          balanceCache,
		Code:             codeCache,
		Storage:          storageCache,
		// Preimages        : preimagesCache,
		// NextAccounts     : nextAccountsCache,
		// NextSlots        : nextSlotsCache,
	}, nil
}

// GetMPTTraces retrieves all MPT traces including storage and contract trie traces
func (mt *MPTTracer) GetMPTTraces() (MPTTraces, error) {
	contractTrie := execution.NewContractTrie(mt.ContractSparseTrie)
	curRoot := mt.ContractSparseTrie.RootHash()

	storageTracesByAccount := make(map[types.Address][]StorageTrieUpdateTrace)
	for addr := range mt.updatedAccounts {
		// contractTrie.SetRootHash() affects underlying ContractSpa rseTrie, so we need to change
		// it each time we switch between current and initial tries
		if err := contractTrie.SetRootHash(mt.initialContractRoot); err != nil {
			return MPTTraces{}, err
		}
		initialSmartContract, err := contractTrie.Fetch(addr.Hash())
		if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
			return MPTTraces{}, err
		}

		if err := contractTrie.SetRootHash(curRoot); err != nil {
			return MPTTraces{}, err
		}
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
	_ = initialRoot

	trie := trieCtor(rawTrie)

	// TODO: use initialRoot to set initial state of trie
	if err := trie.SetRootHash(mpt.EmptyRootHash); err != nil {
		return nil, err
	}
	initialEntries, err := trie.Entries()
	if err != nil {
		return nil, err
	}
	initialEntriesMap := make(map[common.Hash]VPtr, len(initialEntries))
	for _, e := range initialEntries {
		initialEntriesMap[e.Key] = e.Val
	}

	if err := trie.SetRootHash(currentRoot); err != nil {
		return nil, err
	}
	currentEntries, err := trie.Entries()
	if err != nil {
		return nil, err
	}

	// TODO: use initialRoot to set initial state of trie
	if err := trie.SetRootHash(mpt.EmptyRootHash); err != nil {
		return nil, err
	}
	traces := make([]GenericTrieUpdateTrace[VPtr], 0, len(currentEntries)) // can't establish final size here
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

		// ReadOperation plays no role here, could be any
		proof, err := mpt.BuildProof(trie.Reader, e.Key.Bytes(), mpt.ReadOperation)
		if err != nil {
			return nil, err
		}

		slotChangeTrace := GenericTrieUpdateTrace[VPtr]{
			Key:        e.Key,
			RootBefore: trie.RootHash(),
			PathBefore: proof.Nodes,
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

		rootHash, err := trie.Commit()
		if err != nil {
			return nil, err
		}
		if err := trie.SetRootHash(rootHash); err != nil {
			return nil, err
		}

		// ReadOperation plays no role here, could be any
		proof, err = mpt.BuildProof(trie.Reader, e.Key.Bytes(), mpt.ReadOperation)
		if err != nil {
			return nil, err
		}

		slotChangeTrace.RootAfter = rootHash
		slotChangeTrace.PathAfter = proof.Nodes

		traces = append(traces, slotChangeTrace)
	}
	for k, v := range initialEntriesMap {
		// deletion happened

		// ReadOperation plays no role here, could be any
		proof, err := mpt.BuildProof(trie.Reader, k.Bytes(), mpt.ReadOperation)
		if err != nil {
			return nil, err
		}

		slotChangeTrace := GenericTrieUpdateTrace[VPtr]{
			Key:         k,
			RootBefore:  trie.RootHash(),
			PathBefore:  proof.Nodes,
			ValueBefore: v,
			Proof:       proof,
		}

		if err := trie.Delete(k); err != nil {
			return nil, err
		}

		rootHash, err := trie.Commit()
		if err != nil {
			return nil, err
		}
		if err := trie.SetRootHash(rootHash); err != nil {
			return nil, err
		}

		// ReadOperation plays no role here, could be any
		proof, err = mpt.BuildProof(trie.Reader, k.Bytes(), mpt.ReadOperation)
		if err != nil {
			return nil, err
		}

		slotChangeTrace.RootAfter = rootHash
		slotChangeTrace.PathAfter = proof.Nodes
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
	rawMpt := mpt.NewDbMPT(mt.rwTx, mt.shardId, db.ContractTrieTable)
	return getTrieTraces(rawMpt, execution.NewContractTrie, initialRoot, currentRoot)
}

func (mt *MPTTracer) GetRwTx() db.RwTx {
	return mt.rwTx
}
