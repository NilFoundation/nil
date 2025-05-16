package mpt

import (
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/serialization"
	"github.com/NilFoundation/nil/nil/internal/types"
	ethcommon "github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	ethtrie "github.com/ethereum/go-ethereum/trie"
	ethdb "github.com/ethereum/go-ethereum/triedb/database"
)

var EmptyRootHash = common.Hash(ethtypes.EmptyRootHash)

type Reader struct {
	trie   *ethtrie.Trie
	nodeDb ethdb.NodeDatabase
}

type MerklePatriciaTrie struct {
	*Reader
	setter Setter
}

const maxRawKeyLen = 32

func (m *Reader) SetRootHash(root common.Hash) error {
	hash := ethcommon.Hash(root)
	trie, err := ethtrie.New(ethtrie.TrieID(hash), m.nodeDb)
	if err != nil {
		return err
	}
	m.trie = trie
	return nil
}

func (m Reader) RootHash() common.Hash {
	return common.Hash(m.trie.Hash())
}

func (m *Reader) Get(key []byte) ([]byte, error) {
	if len(key) > maxRawKeyLen {
		key = crypto.Keccak256(key)
	}

	val, err := m.trie.Get(key)
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

func (m *MerklePatriciaTrie) SetBatch(keys, values [][]byte) error {
	if len(keys) != len(values) || len(keys) == 0 {
		return ErrInvalidArgSize
	}

	for i, key := range keys {
		if len(key) > maxRawKeyLen {
			key = crypto.Keccak256(key)
		}
		if err := m.trie.Update(key, values[i]); err != nil {
			return err
		}
	}

	return nil
}

func (m *MerklePatriciaTrie) Delete(key []byte) error {
	if len(key) > maxRawKeyLen {
		key = crypto.Keccak256(key)
	}

	if val, err := m.trie.Get(key); err != nil {
		return err
	} else if val == nil {
		return db.ErrKeyNotFound
	}

	return m.trie.Delete(key)
}

func (m *MerklePatriciaTrie) Commit() (common.Hash, error) {
	commitHash, nodeSet := m.trie.Commit(true)
	if nodeSet != nil {
		for hash, blob := range nodeSet.HashSet() {
			if err := m.setter.Set(hash[:], blob); err != nil {
				return common.EmptyHash, err
			}
		}
	}
	return common.Hash(commitHash), nil
}

func NewReader(getter Getter) *Reader {
	nodeDb := newNodeDatabase(getter)
	return &Reader{
		nodeDb: nodeDb,
		trie:   ethtrie.NewEmpty(nodeDb),
	}
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

func (w *nodeDatabaseWrapper) NodeReader(root ethcommon.Hash) (ethdb.NodeReader, error) {
	return &nodeReaderWrapper{getter: w.getter}, nil
}

// Implements database.NodeReader

type nodeReaderWrapper struct {
	getter Getter
}

func (r *nodeReaderWrapper) Node(owner ethcommon.Hash, path []byte, hash ethcommon.Hash) ([]byte, error) {
	return r.getter.Get(hash[:])
}
