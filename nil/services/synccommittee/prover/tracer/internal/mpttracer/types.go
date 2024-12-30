package mpttracer

import (
	"context"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/mpt"
	"github.com/NilFoundation/nil/nil/internal/types"
)

// MPTValueDiff represents a difference in MPT value
type MPTValueDiff struct {
	before types.Uint256
	after  types.Uint256
}

// ContractReader interface for reading contract information
type ContractReader interface {
	GetAccount(ctx context.Context, addr types.Address) (*TracerAccount, mpt.Proof, error)
}

// GenericTrieUpdateTrace is a generic struct for tracking trie updates
type GenericTrieUpdateTrace[T any] struct {
	Key         common.Hash
	RootBefore  common.Hash
	RootAfter   common.Hash
	ValueBefore T
	ValueAfter  T
	Proof       mpt.Proof
}

// StorageTrieUpdateTrace is a type alias for storage trie updates
type StorageTrieUpdateTrace = GenericTrieUpdateTrace[types.Uint256]

// ContractTrieUpdateTrace is a type alias for contract trie updates
type ContractTrieUpdateTrace = GenericTrieUpdateTrace[*types.SmartContract]

// MPTTraces holds storage and contract trie traces
type MPTTraces struct {
	StorageTracesByAccount map[types.Address][]StorageTrieUpdateTrace
	ContractTrieTraces     []ContractTrieUpdateTrace
}
