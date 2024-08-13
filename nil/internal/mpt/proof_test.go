package mpt

import (
	"testing"

	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/stretchr/testify/require"
)

func mptFromData(t *testing.T, data map[string]string) (*MerklePatriciaTrie, map[string][]byte) {
	t.Helper()

	holder := make(map[string][]byte)
	mpt := NewMPT(NewMapSetter(holder), NewReader(NewMapGetter(holder)))
	for k, v := range data {
		require.NoError(t, mpt.Set([]byte(k), []byte(v)))
	}

	return mpt, holder
}

func copyMpt(holder map[string][]byte, mpt *MerklePatriciaTrie) *MerklePatriciaTrie {
	// copy underlying data holder to ensure we not override the data occasionally
	holderCopy := make(map[string][]byte)
	for k, v := range holder {
		holderCopy[k] = v
	}
	tree := NewMPT(NewMapSetter(holderCopy), NewReader(NewMapGetter(holderCopy)))
	tree.SetRootHash(mpt.RootHash())
	return tree
}

func TestReadProof(t *testing.T) {
	t.Parallel()

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
	data := map[string]string{
		string([]byte{0xf, 0xf}):           "val-1",
		string([]byte{0xf, 0xf, 0xa}):      "val-2",
		string([]byte{0xf, 0xe}):           "val-3",
		string([]byte{0xf, 0xe, 0xa}):      "val-4",
		string([]byte{0xf, 0xd}):           "val-5",
		string([]byte{0xf, 0xd, 0xa, 0xa}): "val-6",
	}
	mpt, _ := mptFromData(t, data)

	t.Run("Prove existing keys", func(t *testing.T) {
		t.Parallel()

		for k, v := range data {
			key := []byte(k)
			p, err := BuildProof(mpt.Reader, key, ReadMPTOperation)
			require.NoError(t, err)

			val, err := mpt.Get(key)
			require.NoError(t, err)
			require.Equal(t, string(val), v)

			ok, err := p.VerifyRead(key, val, mpt.RootHash())
			require.NoError(t, err)
			require.True(t, ok)

			ok, err = p.VerifyRead(key, nil, mpt.RootHash())
			require.NoError(t, err)
			require.False(t, ok)
		}
	})

	t.Run("Prove missing keys", func(t *testing.T) {
		t.Parallel()

		verify := func(key []byte) {
			t.Helper()
			p, err := BuildProof(mpt.Reader, key, ReadMPTOperation)
			require.NoError(t, err)

			ok, err := p.VerifyRead(key, nil, mpt.RootHash())
			require.NoError(t, err)
			require.True(t, ok)

			// check that prove fails for non-empty value
			ok, err = p.VerifyRead(key, []byte{0x1}, mpt.RootHash())
			require.NoError(t, err)
			require.False(t, ok)
		}

		verify([]byte{0xf, 0xf, 0xc})

		verify([]byte{0xa})
	})

	t.Run("Prove empty mpt", func(t *testing.T) {
		t.Parallel()

		tree := CreateMerklePatriciaTrie(t)
		key := []byte{0x1}

		p, err := BuildProof(tree.Reader, key, ReadMPTOperation)
		require.NoError(t, err)

		ok, err := p.VerifyRead(key, nil, tree.RootHash())
		require.NoError(t, err)
		require.True(t, ok)
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

	modifyAndBuildProof := func(t *testing.T, mpt *MerklePatriciaTrie, holder map[string][]byte, key []byte, value []byte) (*MerklePatriciaTrie, Proof) {
		t.Helper()

		originalMpt := copyMpt(holder, mpt)
		require.NoError(t, mpt.Set(key, value))

		p, err := BuildProof(originalMpt.Reader, key, SetMPTOperation)
		require.NoError(t, err)

		return originalMpt, p
	}

	t.Run("Prove modify existing", func(t *testing.T) {
		t.Parallel()

		mpt, holder := mptFromData(t, data)

		verify := func(key []byte) {
			t.Helper()
			val := []byte("val-modified")
			valOld := []byte(data[string(key)])
			originalMpt, p := modifyAndBuildProof(t, mpt, holder, key, val)

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

		mpt, holder := mptFromData(t, data)

		verify := func(key []byte) {
			t.Helper()
			val := []byte("val-new")
			originalMpt, p := modifyAndBuildProof(t, mpt, holder, key, val)
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

		tree := CreateMerklePatriciaTrie(t)
		originalMpt := CreateMerklePatriciaTrie(t)
		key := []byte("key")
		val := []byte("val")

		require.NoError(t, tree.Set(key, val))
		p, err := BuildProof(originalMpt.Reader, key, SetMPTOperation)
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

		mpt, holder := mptFromData(t, data)

		key := []byte{0xf}
		originalMpt := copyMpt(holder, mpt)
		require.ErrorIs(t, mpt.Delete(key), db.ErrKeyNotFound)

		p, err := BuildProof(originalMpt.Reader, key, DeleteMPTOperation)
		require.NoError(t, err)

		ok, err := p.VerifyDelete(key, false, originalMpt.RootHash(), mpt.RootHash())
		require.NoError(t, err)
		require.True(t, ok)

		ok, err = p.VerifyDelete(key, true, originalMpt.RootHash(), mpt.RootHash())
		require.NoError(t, err)
		require.False(t, ok)
	})

	t.Run("Delete existing", func(t *testing.T) {
		t.Parallel()

		mpt, holder := mptFromData(t, data)

		verify := func(key []byte) {
			t.Helper()
			originalMpt := copyMpt(holder, mpt)
			require.NoError(t, mpt.Delete(key))

			p, err := BuildProof(originalMpt.Reader, key, DeleteMPTOperation)
			require.NoError(t, err)

			ok, err := p.VerifyDelete(key, true, originalMpt.RootHash(), mpt.RootHash())
			require.NoError(t, err)
			require.True(t, ok)

			ok, err = p.VerifyDelete(key, false, originalMpt.RootHash(), mpt.RootHash())
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

		holder := make(map[string][]byte)
		mpt := NewMPT(NewMapSetter(holder), NewReader(NewMapGetter(holder)))
		require.NoError(t, mpt.Set(key, val))

		originalMpt := copyMpt(holder, mpt)

		require.NoError(t, mpt.Delete(key))

		p, err := BuildProof(originalMpt.Reader, key, DeleteMPTOperation)
		require.NoError(t, err)

		ok, err := p.VerifyDelete(key, true, originalMpt.RootHash(), mpt.RootHash())
		require.NoError(t, err)
		require.True(t, ok)
	})
}

func TestProofEncoding(t *testing.T) {
	t.Parallel()

	mpt := CreateMerklePatriciaTrie(t)
	data := map[string]string{
		string([]byte{0xf, 0xf}):           "val-1",
		string([]byte{0xf, 0xf, 0xa}):      "val-2",
		string([]byte{0xf, 0xe}):           "val-3",
		string([]byte{0xf, 0xe, 0xa}):      "val-4",
		string([]byte{0xf, 0xd}):           "val-5",
		string([]byte{0xf, 0xd, 0xa, 0xa}): "val-6",
	}

	for k, v := range data {
		require.NoError(t, mpt.Set([]byte(k), []byte(v)))
	}

	p, err := BuildProof(mpt.Reader, []byte{0xf, 0xd, 0xa, 0xa}, ReadMPTOperation)
	require.NoError(t, err)

	encoded, err := p.Encode()
	require.NoError(t, err)

	decoded, err := DecodeProof(encoded)
	require.NoError(t, err)

	require.Equal(t, p.operation, decoded.operation)
	require.Equal(t, p.key, decoded.key)
	require.Len(t, decoded.pathToNode, len(p.pathToNode))
	for i, n := range p.pathToNode {
		require.Equal(t, n, decoded.pathToNode[i])
	}
}
