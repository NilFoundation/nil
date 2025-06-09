package jsonrpc

import (
	"context"
	"errors"
	"fmt"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/mpt"
	"github.com/NilFoundation/nil/nil/internal/types"
	rawapitypes "github.com/NilFoundation/nil/nil/services/rpc/rawapi/types"
	"github.com/NilFoundation/nil/nil/services/rpc/transport"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

// GetBalance implements eth_getBalance. Returns the balance of an account for a given address.
func (api *APIImplRo) GetBalance(
	ctx context.Context,
	address types.Address,
	blockNrOrHash transport.BlockNumberOrHash,
) (*hexutil.Big, error) {
	balance, err := api.rawapi.GetBalance(ctx, address, toBlockReference(blockNrOrHash))
	if err != nil {
		return nil, err
	}
	return (*hexutil.Big)(balance.ToBig()), nil
}

// GetTokens implements eth_getTokens. Returns the balance of all tokens of account for a given address.
func (api *APIImplRo) GetTokens(
	ctx context.Context,
	address types.Address,
	blockNrOrHash transport.BlockNumberOrHash,
) (map[types.TokenId]types.Value, error) {
	return api.rawapi.GetTokens(ctx, address, toBlockReference(blockNrOrHash))
}

// GetTransactionCount implements eth_getTransactionCount.
// Returns the number of transactions sent from an address (the nonce / seqno).
func (api *APIImplRo) GetTransactionCount(
	ctx context.Context,
	address types.Address,
	blockNrOrHash transport.BlockNumberOrHash,
) (hexutil.Uint64, error) {
	value, err := api.rawapi.GetTransactionCount(ctx, address, toBlockReference(blockNrOrHash))
	if err != nil {
		return 0, err
	}
	return hexutil.Uint64(value), nil
}

// GetCode implements eth_getCode. Returns the byte code at a given address (if it's a smart contract).
func (api *APIImplRo) GetCode(
	ctx context.Context,
	address types.Address,
	blockNrOrHash transport.BlockNumberOrHash,
) (hexutil.Bytes, error) {
	code, err := api.rawapi.GetCode(ctx, address, toBlockReference(blockNrOrHash))
	if err != nil {
		return nil, err
	}
	return hexutil.Bytes(code), nil
}

// GetProof implements eth_getProof. For more info refer to `EthProof`.
func (api *APIImplRo) GetProof(
	ctx context.Context,
	address types.Address,
	storageKeys []common.Hash,
	blockNrOrHash transport.BlockNumberOrHash,
) (*EthProof, error) {
	// Fetch the smart contract data
	smartContract, err := api.rawapi.GetContract(ctx, address, toBlockReference(blockNrOrHash))
	if err != nil {
		return nil, fmt.Errorf("failed to get contract: %w", err)
	}

	// Process account proof
	accountProofBytes, err := extractAccountProofBytes(smartContract.ProofEncoded)
	if err != nil {
		return nil, fmt.Errorf("failed to extract account proof: %w", err)
	}

	// Prepare storage data for trie
	keys, values := extractStorageKeyValues(smartContract.Storage)

	// Build storage trie
	trie, err := buildStorageTrie(keys, values)
	if err != nil {
		return nil, fmt.Errorf("failed to build storage trie: %w", err)
	}

	// Generate storage proofs for each requested key
	storageProofs, err := generateStorageProofs(trie.Reader, storageKeys)
	if err != nil {
		return nil, fmt.Errorf("failed to generate storage proofs: %w", err)
	}

	// Create the basic proof result
	result := &EthProof{
		Address:      address,
		AccountProof: fromBytesSlice(accountProofBytes),
		StorageProof: storageProofs,
	}

	// If contract data is available, add contract details
	if len(smartContract.ContractBytes) > 0 {
		if err := addContractDetailsToProof(result, smartContract.ContractBytes); err != nil {
			return nil, fmt.Errorf("failed to add contract details: %w", err)
		}
	}

	return result, nil
}

// extractAccountProofBytes decodes and extracts the account proof bytes
func extractAccountProofBytes(proofEncoded []byte) ([][]byte, error) {
	proof, err := mpt.DecodeProof(proofEncoded)
	if err != nil {
		return nil, err
	}

	return proof.ToBytesSlice(), nil
}

// extractStorageKeyValues extracts keys and values from storage map
func extractStorageKeyValues(storage map[common.Hash]types.Uint256) ([]common.Hash, []*types.Uint256) {
	keys := make([]common.Hash, 0, len(storage))
	values := make([]*types.Uint256, 0, len(storage))

	for key, val := range storage {
		localVal := val // Create a local copy to avoid pointer issues
		keys = append(keys, key)
		values = append(values, &localVal)
	}

	return keys, values
}

// buildStorageTrie creates and populates a storage trie with the given keys and values
func buildStorageTrie(keys []common.Hash, values []*types.Uint256) (*mpt.MerklePatriciaTrie, error) {
	trie := mpt.NewInMemMPT()
	storageTrie := execution.NewStorageTrie(trie)

	if err := storageTrie.UpdateBatch(keys, values); err != nil {
		return nil, err
	}

	return trie, nil
}

// generateStorageProofs creates proofs for each requested storage key
func generateStorageProofs(trie *mpt.Reader, storageKeys []common.Hash) ([]StorageProof, error) {
	storageProofs := make([]StorageProof, 0, len(storageKeys))

	for _, key := range storageKeys {
		// Get value for the key
		value, err := trie.Get(key.Bytes())
		if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
			return nil, err
		}

		// Build proof for the key
		proof, err := mpt.BuildProof(trie, key.Bytes(), mpt.ReadOperation)
		if err != nil {
			return nil, err
		}

		// Convert proof to bytes
		proofBytesSlice := proof.ToBytesSlice()

		// Create storage proof for the key
		storageProof := StorageProof{
			Key:   hexutil.Big(*key.Big()),
			Value: (hexutil.Big)(*common.BytesToHash(value).Big()),
		}

		// Add proof bytes if available
		if len(proofBytesSlice) != 0 {
			storageProof.Proof = fromBytesSlice(proofBytesSlice)
		}

		storageProofs = append(storageProofs, storageProof)
	}

	return storageProofs, nil
}

// addContractDetailsToProof adds smart contract details to the proof result
func addContractDetailsToProof(result *EthProof, contractBytes []byte) error {
	contract := new(types.SmartContract)
	if err := contract.UnmarshalNil(contractBytes); err != nil {
		return err
	}

	result.Balance = contract.Balance
	result.CodeHash = contract.CodeHash
	result.Nonce = contract.Seqno
	result.StorageHash = contract.StorageRoot

	return nil
}

func blockNrToBlockReference(num transport.BlockNumber) rawapitypes.BlockReference {
	var ref rawapitypes.BlockReference
	if num <= 0 {
		ref = rawapitypes.NamedBlockIdentifierAsBlockReference(rawapitypes.NamedBlockIdentifier(num))
	} else {
		ref = rawapitypes.BlockNumberAsBlockReference(types.BlockNumber(num))
	}
	return ref
}

func toBlockReference(blockNrOrHash transport.BlockNumberOrHash) rawapitypes.BlockReference {
	if number, ok := blockNrOrHash.Number(); ok {
		return blockNrToBlockReference(number)
	}
	hash, ok := blockNrOrHash.Hash()
	check.PanicIfNot(ok)
	return rawapitypes.BlockHashAsBlockReference(hash)
}
