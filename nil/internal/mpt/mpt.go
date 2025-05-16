// Package mpt wraps go-ethereum's Merkle Patricia Trie.
package mpt

import (
	ethcommon "github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	ethtrie "github.com/ethereum/go-ethereum/trie"
	"github.com/ethereum/go-ethereum/triedb/database"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/serialization"
	"github.com/NilFoundation/nil/nil/internal/types"
)

var EmptyRootHash = common.Hash(ethtypes.EmptyRootHash)

type Reader struct {
	getter Getter
	root   Reference
}

type MerklePatriciaTrie struct {
	*Reader
	setter Setter
}

const maxRawKeyLen = 32

func (m *Reader) SetRootHash(root common.Hash) {
	m.root = root.Bytes()
}

func (m *Reader) RootHash() common.Hash {
	if m.root == nil || len(m.root) == 0 {
		return EmptyRootHash
	}
	return common.BytesToHash(m.root)
}

func (m *Reader) Get(key []byte) ([]byte, error) {
	if len(key) > maxRawKeyLen {
		key = crypto.Keccak256(key)
	}
	nodeDb := newNodeDatabase(m.getter)
	hash := ethcommon.Hash(m.RootHash())
	trie, err := ethtrie.New(ethtrie.TrieID(hash), nodeDb)
	if err != nil {
		return nil, err
	}
	val, err := trie.Get(key)
	if err != nil {
		return nil, err
	}
	if val == nil {
		return nil, db.ErrKeyNotFound
	}
	return val, nil
}

func (m *MerklePatriciaTrie) Set(key, value []byte) error {
	return m.SetBatch([][]byte{key}, [][]byte{value})
}

func (m *MerklePatriciaTrie) SetBatch(keys [][]byte, values [][]byte) error {
	if len(keys) != len(values) || len(keys) == 0 {
		return ErrInvalidArgSize
	}

	db := newNodeDatabaseWithSet(m.getter, m.setter)
	hash := ethcommon.Hash(m.RootHash())
	trie, err := ethtrie.New(ethtrie.TrieID(hash), db)
	if err != nil {
		return err
	}

	for i := range keys {
		key := keys[i]
		if len(key) > maxRawKeyLen {
			key = crypto.Keccak256(key)
		}
		if err := trie.Update(key, values[i]); err != nil {
			return err
		}
	}

	commitHash, nodeSet := trie.Commit(true)
	m.SetRootHash(common.Hash(commitHash))

	for hash, blob := range nodeSet.HashSet() {
		if err := m.setter.Set(hash[:], blob); err != nil {
			return err
		}
	}

	return nil
}

func (m *MerklePatriciaTrie) Delete(key []byte) error {
	if len(key) > maxRawKeyLen {
		key = crypto.Keccak256(key)
	}
	db := newNodeDatabaseWithSet(m.getter, m.setter)
	hash := ethcommon.Hash(m.RootHash())
	trie, err := ethtrie.New(ethtrie.TrieID(hash), db)
	if err != nil {
		return err
	}
	if err := trie.Delete(key); err != nil {
		return err
	}

	commitHash, nodeSet := trie.Commit(true)
	m.SetRootHash(common.Hash(commitHash))

	for hash, blob := range nodeSet.HashSet() {
		if err := m.setter.Set(hash[:], blob); err != nil {
			return err
		}
	}

	return nil
}

func NewReader(getter Getter) *Reader {
	return &Reader{getter: getter}
}

func NewDbReader(tx db.RoTx, shardId types.ShardId, name db.ShardedTableName) *Reader {
	return NewReader(NewDbGetter(tx, shardId, name))
}

func NewMPT(setter Setter, reader *Reader) *MerklePatriciaTrie {
	return &MerklePatriciaTrie{reader, setter}
}

func NewDbMPT(tx db.RwTx, shardId types.ShardId, name db.ShardedTableName) *MerklePatriciaTrie {
	return NewMPT(NewDbSetter(tx, shardId, name), NewDbReader(tx, shardId, name))
}

func GetEntity[
	T interface {
		~*S
		serialization.NilUnmarshaler
	},
	S any,
](r *Reader, key []byte) (*S, error) {
	data, err := r.Get(key)
	if err != nil {
		return nil, err
	}
	var s S
	return &s, T(&s).UnmarshalNil(data)
}

// Implements database.NodeDatabase

type nodeDatabaseWrapper struct {
	getter Getter
}

func newNodeDatabase(getter Getter) *nodeDatabaseWrapper {
	return &nodeDatabaseWrapper{getter: getter}
}

func newNodeDatabaseWithSet(getter Getter, setter Setter) *nodeDatabaseWithSetWrapper {
	return &nodeDatabaseWithSetWrapper{getter: getter, setter: setter}
}

func (w *nodeDatabaseWrapper) NodeReader(root ethcommon.Hash) (database.NodeReader, error) {
	return &nodeReaderWrapper{getter: w.getter}, nil
}

// Implements database.NodeDatabase for write

type nodeDatabaseWithSetWrapper struct {
	getter Getter
	setter Setter
}

func (w *nodeDatabaseWithSetWrapper) NodeReader(root ethcommon.Hash) (database.NodeReader, error) {
	return &nodeReaderWrapper{getter: w.getter}, nil
}

// Implements database.NodeReader

type nodeReaderWrapper struct {
	getter Getter
}

func (r *nodeReaderWrapper) Node(owner ethcommon.Hash, path []byte, hash ethcommon.Hash) ([]byte, error) {
	return r.getter.Get(hash[:])
}
