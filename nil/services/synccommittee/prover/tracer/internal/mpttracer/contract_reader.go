package mpttracer

import (
	"github.com/NilFoundation/nil/nil/client"
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/mpt"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
	"github.com/NilFoundation/nil/nil/services/rpc/transport"
)

type DeserializedDebugRPCContract struct {
	Contract                types.SmartContract
	ExistenceProof          mpt.Proof
	Code                    types.Code
	StorageTrieEntries      map[common.Hash]types.Uint256
	CurrencyTrieEntries     map[types.CurrencyId]types.Value
	AsyncContextTrieEntries map[types.MessageIndex]types.AsyncContext
}

func deserializeDebugRPCContract(debugRPCContract *jsonrpc.DebugRPCContract) (*DeserializedDebugRPCContract, error) {
	contract := new(types.SmartContract)
	if err := contract.UnmarshalSSZ(debugRPCContract.Contract); err != nil {
		return nil, err
	}

	existenceProof, err := mpt.DecodeProof(debugRPCContract.Proof)
	if err != nil {
		return nil, err
	}

	return &DeserializedDebugRPCContract{
		Contract:                *contract,
		ExistenceProof:          existenceProof,
		Code:                    types.Code(debugRPCContract.Code),
		StorageTrieEntries:      debugRPCContract.Storage,
		CurrencyTrieEntries:     debugRPCContract.Currencies,
		AsyncContextTrieEntries: debugRPCContract.AsyncContext,
	}, nil
}

// DebugApiContractReader implements ContractReader for debug API
type DebugApiContractReader struct {
	client           client.Client
	shardBlockNumber types.BlockNumber
	rwTx             db.RwTx
	shardId          types.ShardId
}

// NewDebugApiContractReader creates a new DebugApiContractReader
func NewDebugApiContractReader(
	client client.Client,
	shardBlockNumber types.BlockNumber,
	rwTx db.RwTx,
	shardId types.ShardId,
) *DebugApiContractReader {
	return &DebugApiContractReader{
		client:           client,
		shardBlockNumber: shardBlockNumber,
		rwTx:             rwTx,
		shardId:          shardId,
	}
}

// GetRwTx returns the read-write transaction
func (dacr *DebugApiContractReader) GetRwTx() db.RwTx {
	return dacr.rwTx
}

// AppendToJournal is a no-op method to satisfy the interface
func (dacr *DebugApiContractReader) AppendToJournal(je execution.JournalEntry) {}

// GetAccount retrieves an account with its debug information
func (dacr *DebugApiContractReader) GetAccount(addr types.Address) (*TracerAccount, mpt.Proof, error) {
	debugRPCContract, err := dacr.client.GetDebugContract(addr, transport.BlockNumber(dacr.shardBlockNumber))
	if err != nil || debugRPCContract == nil {
		return nil, mpt.Proof{}, err
	}

	debugContract, err := deserializeDebugRPCContract(debugRPCContract)
	if err != nil {
		return nil, mpt.Proof{}, err
	}

	err = insertTrieValues(
		dacr.rwTx,
		dacr.shardId,
		debugContract.StorageTrieEntries,
		execution.NewDbStorageTrie,
	)
	if err != nil {
		return nil, mpt.Proof{}, err
	}

	err = insertTrieValues(
		dacr.rwTx,
		dacr.shardId,
		debugContract.CurrencyTrieEntries,
		execution.NewDbCurrencyTrie,
	)
	if err != nil {
		return nil, mpt.Proof{}, err
	}

	err = insertTrieValues(
		dacr.rwTx,
		dacr.shardId,
		debugContract.AsyncContextTrieEntries,
		execution.NewDbAsyncContextTrie,
	)
	if err != nil {
		return nil, mpt.Proof{}, err
	}

	err = db.WriteCode(dacr.rwTx, dacr.shardId, debugContract.Code.Hash(), debugContract.Code)
	if err != nil {
		return nil, mpt.Proof{}, err
	}

	accountState, err := NewTracerAccount(
		dacr,
		debugContract.Contract.Address,
		&debugContract.Contract,
	)
	if err != nil {
		return nil, mpt.Proof{}, err
	}

	return accountState, debugContract.ExistenceProof, nil
}

// Generic function to insert key-value trie pairs into db
func insertTrieValues[K comparable, V any, VPtr execution.MPTValue[V]](
	tx db.RwTx,
	shardId types.ShardId,
	entries map[K]V,
	trieCreator func(db.RwTx, types.ShardId) *execution.BaseMPT[K, V, VPtr],
) error {
	if len(entries) == 0 {
		return nil
	}

	trie := trieCreator(tx, shardId)

	keys := make([]K, 0, len(entries))
	values := make([]VPtr, 0, len(entries))

	for key, val := range entries {
		keys = append(keys, key)
		values = append(values, &val)
	}

	return trie.UpdateBatch(keys, values)
}

// Ensure DebugApiContractReader implements ContractReader
var _ ContractReader = &DebugApiContractReader{}
