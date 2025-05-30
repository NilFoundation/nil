package mpt_test

import (
	"sync"
	"testing"

	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/mpt"
	"github.com/stretchr/testify/require"
)

/*
	             R
		         |
	            Ext
	             |
	    _________Br________
	   /          |        \

Br[val-1]   Br[val-3]  Br[val-5]

	|            |          |

Leaf[val-2] Leaf[val-4]  Leaf[val-6]
*/
var defaultMPTData = map[string]string{
	string([]byte{0xf, 0xf}):           "val-1",
	string([]byte{0xf, 0xf, 0xa}):      "val-2",
	string([]byte{0xf, 0xe}):           "val-3",
	string([]byte{0xf, 0xe, 0xa}):      "val-4",
	string([]byte{0xf, 0xd}):           "val-5",
	string([]byte{0xf, 0xd, 0xa, 0xa}): "val-6",
}

func mptFromData(t *testing.T, data map[string]string) *mpt.MerklePatriciaTrie {
	t.Helper()

	mpt := mpt.NewInMemMPT()
	for k, v := range data {
		require.NoError(t, mpt.Set([]byte(k), []byte(v)))
	}
	return mpt
}

var mutex sync.Mutex

func copyMpt(t *testing.T, trie *mpt.MerklePatriciaTrie) *mpt.MerklePatriciaTrie {
	t.Helper()

	// Locking is necessary to avoid concurrent access to the trie.
	// Even read operations can modify the trie structure.
	mutex.Lock()
	defer mutex.Unlock()

	copied := mpt.NewInMemMPT()
	for k, v := range trie.Iterate() {
		require.NoError(t, copied.Set(k, v))
	}
	require.Equal(t, trie.RootHash(), copied.RootHash())
	return copied
}

func TestReadProof(t *testing.T) {
	t.Parallel()

	data := defaultMPTData

	t.Run("Prove existing keys", func(t *testing.T) {
		t.Parallel()

		trie := mptFromData(t, data)
		for k, v := range data {
			key := []byte(k)
			p, err := mpt.BuildProof(trie.Reader, key, mpt.ReadOperation)
			require.NoError(t, err)

			val, err := trie.Get(key)
			require.NoError(t, err)
			require.Equal(t, string(val), v)

			ok, err := p.VerifyRead(key, val, trie.RootHash())
			require.NoError(t, err)
			require.True(t, ok)

			ok, err = p.VerifyRead(key, nil, trie.RootHash())
			require.NoError(t, err)
			require.False(t, ok)
		}
	})

	t.Run("Prove missing keys", func(t *testing.T) {
		t.Parallel()

		trie := mptFromData(t, data)
		verify := func(key []byte) {
			t.Helper()
			p, err := mpt.BuildProof(trie.Reader, key, mpt.ReadOperation)
			require.NoError(t, err)

			ok, err := p.VerifyRead(key, nil, trie.RootHash())
			require.NoError(t, err)
			require.True(t, ok)

			// check that prove fails for non-empty value
			ok, err = p.VerifyRead(key, []byte{0x1}, trie.RootHash())
			require.NoError(t, err)
			require.False(t, ok)
		}

		verify([]byte{0xf, 0xf, 0xc})

		verify([]byte{0xa})
	})

	t.Run("Prove empty mpt", func(t *testing.T) {
		t.Parallel()

		tree := mpt.NewInMemMPT()
		key := []byte{0x1}

		p, err := mpt.BuildProof(tree.Reader, key, mpt.ReadOperation)
		require.NoError(t, err)

		ok, err := p.VerifyRead(key, nil, tree.RootHash())
		require.NoError(t, err)
		require.True(t, ok)
	})
}

func TestSparseMPT(t *testing.T) {
	t.Parallel()

	data := defaultMPTData
	trie := mptFromData(t, data)

	filter := func(key string) bool {
		return len(key) < 3
	}

	sparseHolder := mpt.NewInMemHolder()
	sparse := mpt.NewMPTFromMap(sparseHolder)
	for k := range data {
		if !filter(k) {
			continue
		}

		p, err := mpt.BuildProof(trie.Reader, []byte(k), mpt.ReadOperation)
		require.NoError(t, err)

		require.NoError(t, mpt.PopulateMptWithProof(sparse, &p))
		require.NoError(t, sparse.SetRootHash(trie.RootHash()))

		val, err := sparse.Get([]byte(k))
		require.NoError(t, err)
		require.Equal(t, data[k], string(val))
	}

	t.Run("Check original keys", func(t *testing.T) {
		t.Parallel()

		sparse := copyMpt(t, sparse)
		for k, v := range data {
			if !filter(k) {
				continue
			}

			val, err := sparse.Get([]byte(k))
			require.NoError(t, err)
			require.Equal(t, string(val), v)
		}
	})

	t.Run("Check missing keys", func(t *testing.T) {
		t.Parallel()

		sparse := copyMpt(t, sparse)
		for _, k := range [][]byte{
			{0xf, 0xf, 0xc},
			{0xa},
		} {
			val, err := sparse.Get(k)
			require.ErrorIs(t, err, db.ErrKeyNotFound)
			require.Nil(t, val)
		}
	})
}

func TestSetProof(t *testing.T) {
	t.Parallel()

	/*
						 R
						 |
						Ext
						 |
				_________Br________
				 /          |        \
			  Br[val-1]   Br[val-4]  Br[val-6]
				|            |          |
			   Ext       Leaf[val-5]  Leaf[val-7]
				|
			  __Br______
			 /          \
		   Leaf[val-2]  Leaf[val-3]
	*/
	data := map[string]string{
		string([]byte{0xf, 0xf}):           "val-1",
		string([]byte{0xf, 0xf, 0xa, 0xb}): "val-2",
		string([]byte{0xf, 0xf, 0xa, 0xc}): "val-3",
		string([]byte{0xf, 0xe}):           "val-4",
		string([]byte{0xf, 0xe, 0xa}):      "val-5",
		string([]byte{0xf, 0xd}):           "val-6",
		string([]byte{0xf, 0xd, 0xa, 0xa}): "val-7",
	}

	modifyAndBuildProof := func(
		t *testing.T,
		trie *mpt.MerklePatriciaTrie,
		key []byte,
		value []byte,
	) (*mpt.MerklePatriciaTrie, mpt.Proof) {
		t.Helper()

		originalMpt := copyMpt(t, trie)
		require.NoError(t, trie.Set(key, value))

		rootHash, err := trie.Commit()
		require.NoError(t, err)
		require.NoError(t, trie.SetRootHash(rootHash))

		p, err := mpt.BuildProof(originalMpt.Reader, key, mpt.SetOperation)
		require.NoError(t, err)

		return originalMpt, p
	}

	t.Run("Prove modify existing", func(t *testing.T) {
		t.Parallel()

		mpt := mptFromData(t, data)

		verify := func(key []byte) {
			t.Helper()
			val := []byte("val-modified")
			valOld := []byte(data[string(key)])
			originalMpt, p := modifyAndBuildProof(t, mpt, key, val)

			// check with correct value
			ok, err := p.VerifySet(key, val, originalMpt.RootHash(), mpt.RootHash())
			require.NoError(t, err)
			require.True(t, ok)

			// check with wrong value
			ok, err = p.VerifySet(key, valOld, originalMpt.RootHash(), mpt.RootHash())
			require.NoError(t, err)
			require.False(t, ok)
		}

		// here we pick two keys: 1st is stored inside BranchNode and 2nd is inside LeafNode
		verify([]byte{0xf, 0xf})

		verify([]byte{0xf, 0xf, 0xa, 0xb})
	})

	t.Run("Prove new key", func(t *testing.T) {
		t.Parallel()

		mpt := mptFromData(t, data)

		verify := func(key []byte) {
			t.Helper()
			val := []byte("val-new")
			originalMpt, p := modifyAndBuildProof(t, mpt, key, val)
			ok, err := p.VerifySet(key, val, originalMpt.RootHash(), mpt.RootHash())
			require.NoError(t, err)
			require.True(t, ok)
		}

		// new branch for BranchNode without value
		verify([]byte{0xf, 0xc})

		// add sibling for existing leaf
		verify([]byte{0xf, 0xe, 0xb})

		// add sibling for existing extension node
		verify([]byte{0xf, 0xf, 0xb})
	})

	t.Run("Prove add to empty tree", func(t *testing.T) {
		t.Parallel()

		tree := mpt.NewInMemMPT()
		originalMpt := mpt.NewInMemMPT()
		key := []byte("key")
		val := []byte("val")

		require.NoError(t, tree.Set(key, val))
		p, err := mpt.BuildProof(originalMpt.Reader, key, mpt.SetOperation)
		require.NoError(t, err)

		ok, err := p.VerifySet(key, val, originalMpt.RootHash(), tree.RootHash())
		require.NoError(t, err)
		require.True(t, ok)

		ok, err = p.VerifySet(key, []byte("val-wrong"), originalMpt.RootHash(), tree.RootHash())
		require.NoError(t, err)
		require.False(t, ok)
	})
}

func TestDeleteProof(t *testing.T) {
	t.Parallel()

	/*
	                 R
	    	         |
	                Ext
	                 |
	        _________Br________
	       /          |        \
	    Br[val-1]   Br[val-4]  Br[val-6]
	      |            |          |
	     Ext       Leaf[val-5]  Leaf[val-7]
	      |
	    __Br______
	   /          \
	 Leaf[val-2]  Leaf[val-3]

	*/
	data := map[string]string{
		string([]byte{0xf, 0xf}):           "val-1",
		string([]byte{0xf, 0xf, 0xa, 0xb}): "val-2",
		string([]byte{0xf, 0xf, 0xa, 0xc}): "val-3",
		string([]byte{0xf, 0xe}):           "val-4",
		string([]byte{0xf, 0xe, 0xa}):      "val-5",
		string([]byte{0xf, 0xd}):           "val-6",
		string([]byte{0xf, 0xd, 0xa, 0xa}): "val-7",
	}

	t.Run("Delete non existing", func(t *testing.T) {
		t.Parallel()

		trie := mptFromData(t, data)

		key := []byte{0xf}
		originalMpt := copyMpt(t, trie)
		require.ErrorIs(t, trie.Delete(key), db.ErrKeyNotFound)

		p, err := mpt.BuildProof(originalMpt.Reader, key, mpt.DeleteOperation)
		require.NoError(t, err)

		ok, err := p.VerifyDelete(key, false, originalMpt.RootHash(), trie.RootHash())
		require.NoError(t, err)
		require.True(t, ok)

		ok, err = p.VerifyDelete(key, true, originalMpt.RootHash(), trie.RootHash())
		require.NoError(t, err)
		require.False(t, ok)
	})

	t.Run("Delete existing", func(t *testing.T) {
		t.Parallel()

		trie := mptFromData(t, data)

		verify := func(key []byte) {
			t.Helper()
			originalMpt := copyMpt(t, trie)
			require.NoError(t, trie.Delete(key))

			rootHash, err := trie.Commit()
			require.NoError(t, err)
			require.NoError(t, trie.SetRootHash(rootHash))

			p, err := mpt.BuildProof(originalMpt.Reader, key, mpt.DeleteOperation)
			require.NoError(t, err)

			ok, err := p.VerifyDelete(key, true, originalMpt.RootHash(), trie.RootHash())
			require.NoError(t, err)
			require.True(t, ok)

			ok, err = p.VerifyDelete(key, false, originalMpt.RootHash(), trie.RootHash())
			require.NoError(t, err)
			require.False(t, ok)
		}

		verify([]byte{0xf, 0xf, 0xa, 0xb})

		verify([]byte{0xf, 0xf})

		verify([]byte{0xf, 0xe, 0xa})
	})

	t.Run("Delete last key", func(t *testing.T) {
		t.Parallel()

		key := []byte{0xf}
		val := []byte("val")

		trie := mpt.NewInMemMPT()
		require.NoError(t, trie.Set(key, val))

		rootHash, err := trie.Commit()
		require.NoError(t, err)
		require.NoError(t, trie.SetRootHash(rootHash))

		originalMpt := copyMpt(t, trie)

		require.NoError(t, trie.Delete(key))

		rootHash, err = trie.Commit()
		require.NoError(t, err)
		require.NoError(t, trie.SetRootHash(rootHash))

		p, err := mpt.BuildProof(originalMpt.Reader, key, mpt.DeleteOperation)
		require.NoError(t, err)

		ok, err := p.VerifyDelete(key, true, originalMpt.RootHash(), trie.RootHash())
		require.NoError(t, err)
		require.True(t, ok)
	})
}

func TestProofEncoding(t *testing.T) {
	t.Parallel()

	data := defaultMPTData
	trie := mptFromData(t, data)

	p, err := mpt.BuildProof(trie.Reader, []byte{0xf, 0xd, 0xa, 0xa}, mpt.ReadOperation)
	require.NoError(t, err)

	encoded, err := p.Encode()
	require.NoError(t, err)

	decoded, err := mpt.DecodeProof(encoded)
	require.NoError(t, err)

	require.Equal(t, p.Operation, decoded.Operation)
	require.Equal(t, p.Key, decoded.Key)
	require.Len(t, decoded.Nodes, len(p.Nodes))
	for i, n := range p.Nodes {
		require.Equal(t, n, decoded.Nodes[i])
	}
}
