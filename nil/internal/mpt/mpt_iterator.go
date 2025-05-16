package mpt

import (
	"iter"

	ethcommon "github.com/ethereum/go-ethereum/common"
	ethtrie "github.com/ethereum/go-ethereum/trie"
)

func (m *Reader) Iterate() iter.Seq2[[]byte, []byte] {
	type Yield = func([]byte, []byte) bool
	return func(yield Yield) {
		db := newNodeDatabase(m.getter)
		hash := ethcommon.Hash(m.RootHash())
		trie, err := ethtrie.New(ethtrie.TrieID(hash), db)
		if err != nil {
			return
		}

		it, err := trie.NodeIterator(nil)
		if err != nil {
			return
		}
		iterator := ethtrie.NewIterator(it)
		for iterator.Next() {
			if !yield(iterator.Key, iterator.Value) {
				return
			}
		}
	}
}
