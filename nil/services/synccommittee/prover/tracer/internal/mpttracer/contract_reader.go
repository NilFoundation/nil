package mpttracer

import (
	"context"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/mpt"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
	"github.com/NilFoundation/nil/nil/services/rpc/transport"
	"github.com/NilFoundation/nil/nil/services/synccommittee/prover/tracer/api"
)

type DeserializedDebugRPCContract struct {
	Contract                types.SmartContract
	ExistenceProof          mpt.Proof
	Code                    types.Code
	StorageTrieEntries      map[common.Hash]types.Uint256
	TokenTrieEntries        map[types.TokenId]types.Value
	AsyncContextTrieEntries map[types.TransactionIndex]types.AsyncContext
}

func deserializeDebugRPCContract(debugRPCContract *jsonrpc.DebugRPCContract) (*DeserializedDebugRPCContract, error) {
	contract := new(types.SmartContract)
	if err := contract.UnmarshalSSZ(debugRPCContract.Contract); err != nil {
		return nil, err
	}

	return &DeserializedDebugRPCContract{
		Contract:                *contract,
		Code:                    types.Code(debugRPCContract.Code),
		StorageTrieEntries:      debugRPCContract.Storage,
		TokenTrieEntries:        debugRPCContract.Tokens,
		AsyncContextTrieEntries: debugRPCContract.AsyncContext,
	}, nil
}

// DebugApiContractReader implements ContractReader for debug API
type DebugApiContractReader struct {
	client           api.RpcClient
	shardBlockNumber types.BlockNumber
	rwTx             db.RwTx
	shardId          types.ShardId
}

// Ensure DebugApiContractReader implements ContractReader
var _ ContractReader = (*DebugApiContractReader)(nil)

// NewDebugApiContractReader creates a new DebugApiContractReader
func NewDebugApiContractReader(
	client api.RpcClient,
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

// GetAccount retrieves an account with its debug information. If not such contract at the given address,
// nil and proof of absence are returned
func (dacr *DebugApiContractReader) GetAccount(
	ctx context.Context,
	addr types.Address,
) (*types.SmartContract, mpt.Proof, error) {
	debugRPCContract, err := dacr.client.GetDebugContract(ctx, addr, transport.BlockNumber(dacr.shardBlockNumber))
	if err != nil || debugRPCContract == nil {
		return nil, mpt.Proof{}, err
	}

	proof, err := mpt.DecodeProof(debugRPCContract.Proof)
	if err != nil {
		return nil, mpt.Proof{}, err
	}

	if len(debugRPCContract.Contract) == 0 {
		// no such contract, absence proof is still provided
		return nil, proof, nil
	}

	debugContract, err := deserializeDebugRPCContract(debugRPCContract)
	if err != nil {
		return nil, mpt.Proof{}, err
	}
	debugContract.ExistenceProof = proof

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
		debugContract.TokenTrieEntries,
		execution.NewDbTokenTrie,
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

	return &debugContract.Contract, debugContract.ExistenceProof, nil
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
