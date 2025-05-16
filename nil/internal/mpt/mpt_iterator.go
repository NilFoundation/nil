package mpt

import (
	"iter"

	ethtrie "github.com/ethereum/go-ethereum/trie"
)

func (m *Reader) Iterate() iter.Seq2[[]byte, []byte] {
	type Yield = func([]byte, []byte) bool
	return func(yield Yield) {
		it, err := m.trie.NodeIterator(nil)
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
