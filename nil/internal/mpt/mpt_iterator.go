package mpt

import (
	"iter"

	ethtrie "github.com/ethereum/go-ethereum/trie"
)

func (m *Reader) Iterate() iter.Seq2[[]byte, []byte] {
	return m.IterateFromKey(nil)
}

// IterateFromKey returns an iterator that yields all key-value pairs starting from the given key (inclusive).
func (m *Reader) IterateFromKey(start []byte) iter.Seq2[[]byte, []byte] {
	type Yield = func([]byte, []byte) bool
	return func(yield Yield) {
		it, err := m.trie.NodeIterator(start)
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
