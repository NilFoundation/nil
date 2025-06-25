package execution

import (
	"errors"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/types"
)

// GetHashFn returns a GetHashFunc which retrieves block hashes by number
func getHashFn(es *ExecutionState, ref *types.Block) func(n uint64) (common.Hash, error) {
	// Cache will initially contain [refHash.parent],
	// Then fill up with [refHash.p, refHash.pp, refHash.ppp, ...]
	var cache []common.Hash
	lastBlockId := uint64(0)
	if ref != nil {
		lastBlockId = ref.Id.Uint64()
	}

	return func(n uint64) (common.Hash, error) {
		if lastBlockId <= n {
			// This situation can happen if we're doing tracing and using
			// block overrides.
			return common.EmptyHash, nil
		}
		// If there's no hash cache yet, make one
		if len(cache) == 0 {
			cache = append(cache, ref.PrevBlock)
		}
		if idx := ref.Id.Uint64() - n - 1; idx < uint64(len(cache)) {
			return cache[idx], nil
		}
		// No luck in the cache, but we can start iterating from the last element we already know
		lastKnownHash := cache[len(cache)-1]

		for {
			b, err := es.blockAccessor.GetByHash(es.tx, es.ShardId, lastKnownHash)
			if errors.Is(err, db.ErrKeyNotFound) {
				break
			}
			if err != nil {
				return common.EmptyHash, err
			}

			cache = append(cache, b.PrevBlock)
			lastKnownHash = b.PrevBlock
			lastKnownNumber := b.Id.Uint64() - 1
			if n == lastKnownNumber {
				return lastKnownHash, nil
			}
		}
		return common.EmptyHash, nil
	}
}
