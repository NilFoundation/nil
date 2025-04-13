package mpttracer

import (
	"context"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/mpt"
	"github.com/NilFoundation/nil/nil/internal/types"
)

// ContractReader interface for reading contract information
type ContractReader interface {
	GetAccount(ctx context.Context, addr types.Address) (*types.SmartContract, mpt.Proof, error)
}

// GenericTrieUpdateTrace is a generic struct for tracking trie updates
type GenericTrieUpdateTrace[T any] struct {
	Key         common.Hash
	RootBefore  common.Hash
	RootAfter   common.Hash
	ValueBefore T
	ValueAfter  T
	Proof       mpt.Proof
	PathBefore  mpt.ProofPath
	PathAfter   mpt.ProofPath
}

// StorageTrieUpdateTrace is a type alias for storage trie updates
type StorageTrieUpdateTrace = GenericTrieUpdateTrace[*types.Uint256]

// ContractTrieUpdateTrace is a type alias for contract trie updates
type ContractTrieUpdateTrace = GenericTrieUpdateTrace[*types.SmartContract]

// MPTTraces holds storage and contract trie traces
type MPTTraces struct {
	StorageTracesByAccount map[types.Address][]StorageTrieUpdateTrace
	ContractTrieTraces     []ContractTrieUpdateTrace
}
