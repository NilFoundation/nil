package execution

import (
	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/types"
)

// GetHashFn returns a GetHashFunc which retrieves block hashes by number
func getHashFn(es *ExecutionState, ref *types.Block) func(n uint64) common.Hash {
	// Cache will initially contain [refHash.parent],
	// Then fill up with [refHash.p, refHash.pp, refHash.ppp, ...]
	var cache []common.Hash
	lastBlockId := uint64(0)
	if ref != nil {
		lastBlockId = ref.Id
	}

	return func(n uint64) common.Hash {
		if lastBlockId <= n {
			// This situation can happen if we're doing tracing and using
			// block overrides.
			return common.Hash{}
		}
		// If there's no hash cache yet, make one
		if len(cache) == 0 {
			cache = append(cache, ref.PrevBlock)
		}
		if idx := ref.Id - n - 1; idx < uint64(len(cache)) {
			return cache[idx]
		}
		// No luck in the cache, but we can start iterating from the last element we already know
		lastKnownHash := cache[len(cache)-1]

		for {
			header := db.ReadBlock(es.tx, es.ShardId, lastKnownHash)
			if header == nil {
				break
			}
			cache = append(cache, header.PrevBlock)
			lastKnownHash = header.PrevBlock
			lastKnownNumber := header.Id - 1
			if n == lastKnownNumber {
				return lastKnownHash
			}
		}
		return common.Hash{}
	}
}
