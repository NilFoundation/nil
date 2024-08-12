package mpt

import (
	"encoding/binary"
	"math/rand"
	"testing"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func getValue(t *testing.T, trie *MerklePatriciaTrie, key []byte) []byte {
	t.Helper()

	value, err := trie.Get(key)
	require.NoError(t, err)
	return value
}

func CreateMerklePatriciaTrie(t *testing.T) *MerklePatriciaTrie {
	t.Helper()

	holder := make(map[string][]byte)
	return NewMPT(NewMapSetter(holder), NewReader(NewMapGetter(holder)))
}

func TestInsertGetOneShort(t *testing.T) {
	t.Parallel()

	trie := CreateMerklePatriciaTrie(t)
	key := []byte("key")
	value := []byte("value")

	require.NoError(t, trie.Set(key, value))
	assert.Equal(t, value, getValue(t, trie, key))

	gotValue, err := trie.Get([]byte("wrong_key"))
	require.Error(t, err)
	assert.Empty(t, gotValue)
}

func TestInsertGetOneLong(t *testing.T) {
	t.Parallel()

	trie := CreateMerklePatriciaTrie(t)

	key := []byte("key_0000000000000000000000000000000000000000000000000000000000000000")
	value := []byte("value_0000000000000000000000000000000000000000000000000000000000000000")
	require.NoError(t, trie.Set(key, value))
	require.Equal(t, value, getValue(t, trie, key))
}

func TestInsertGetMany(t *testing.T) {
	t.Parallel()

	trie := CreateMerklePatriciaTrie(t)

	cases := []struct {
		k string
		v string
	}{
		{"do", "verb"},
		{"dog", "puppy"},
		{"doge", "coin"},
		{"horse", "stallion"},
	}

	for _, c := range cases {
		require.NoError(t, trie.Set([]byte(c.k), []byte(c.v)))
	}

	for _, c := range cases {
		assert.Equal(t, []byte(c.v), getValue(t, trie, []byte(c.k)))
	}
}

func TestIterate(t *testing.T) {
	t.Parallel()

	trie := CreateMerklePatriciaTrie(t)
	// Check iteration on the empty trie
	for range trie.Iterate() {
	}

	keys := [][]byte{[]byte("do"), []byte("dog"), []byte("doge"), []byte("horse")}
	values := [][]byte{[]byte("verb"), []byte("puppy"), []byte("coin"), []byte("stallion")}

	for i := range len(keys) {
		require.NoError(t, trie.Set(keys[i], values[i]))
	}

	i := 0
	for _, kv := range trie.Iterate() {
		require.Equal(t, kv.Key, keys[i])
		require.Equal(t, kv.Value, values[i])
		i += 1
	}
	require.Len(t, keys, i)
}

func TestInsertGetLots(t *testing.T) {
	t.Parallel()

	trie := CreateMerklePatriciaTrie(t)
	const size = 100

	var keys [size][]byte
	var values [size][]byte
	for i := range size {
		n := rand.Uint64() //nolint:gosec
		keys[i] = binary.LittleEndian.AppendUint64(keys[i], n)
		values[i] = binary.LittleEndian.AppendUint32(values[i], uint32(i))
	}

	for i, key := range keys {
		require.NoError(t, trie.Set(key, values[i]))
	}

	for i := range keys {
		assert.Equal(t, values[i], getValue(t, trie, keys[i]))
	}
}

func TestDeleteOne(t *testing.T) {
	t.Parallel()

	trie := CreateMerklePatriciaTrie(t)

	require.NoError(t, trie.Set([]byte("key"), []byte("value")))
	require.NoError(t, trie.Delete([]byte("key")))

	value, err := trie.Get([]byte("key"))
	require.Equal(t, value, []byte(nil))
	require.Error(t, err)
}

func TestDeleteMany(t *testing.T) {
	t.Parallel()

	trie := CreateMerklePatriciaTrie(t)

	require.NoError(t, trie.Set([]byte("do"), []byte("verb")))
	require.NoError(t, trie.Set([]byte("dog"), []byte("puppy")))
	require.NoError(t, trie.Set([]byte("doge"), []byte("coin")))
	require.NoError(t, trie.Set([]byte("horse"), []byte("stallion")))

	rootHash := trie.RootHash()

	require.NoError(t, trie.Set([]byte("a"), []byte("aaa")))
	require.NoError(t, trie.Set([]byte("some_key"), []byte("some_value")))
	require.NoError(t, trie.Set([]byte("dodog"), []byte("do_dog")))

	newRootHash := trie.RootHash()

	require.NotEqual(t, rootHash, newRootHash)

	require.NoError(t, trie.Delete([]byte("a")))
	require.NoError(t, trie.Delete([]byte("some_key")))
	require.NoError(t, trie.Delete([]byte("dodog")))

	newRootHash = trie.RootHash()

	require.Equal(t, rootHash, newRootHash)
}

func TestDeleteLots(t *testing.T) {
	t.Parallel()

	trie := CreateMerklePatriciaTrie(t)
	const size = 100

	require.Equal(t, trie.RootHash(), common.EmptyHash)

	var keys [size][]byte
	var values [size][]byte
	for i := range size {
		keys[i] = binary.LittleEndian.AppendUint64(keys[i], rand.Uint64()) //nolint:gosec
		values[i] = binary.LittleEndian.AppendUint32(values[i], uint32(i))
	}

	for i, key := range keys {
		require.NoError(t, trie.Set(key, values[i]))
	}

	require.NotEqual(t, trie.RootHash(), common.EmptyHash)

	for i := range keys {
		require.NoError(t, trie.Delete(keys[i]))
	}

	require.Equal(t, trie.RootHash(), common.EmptyHash)
}

func TestTrieFromOldRoot(t *testing.T) {
	t.Parallel()

	trie := CreateMerklePatriciaTrie(t)

	require.NoError(t, trie.Set([]byte("do"), []byte("verb")))
	require.NoError(t, trie.Set([]byte("dog"), []byte("puppy")))

	rootHash := trie.RootHash()

	require.NoError(t, trie.Delete([]byte("dog")))
	require.NoError(t, trie.Set([]byte("do"), []byte("not_a_verb")))

	// New
	require.Equal(t, []byte("not_a_verb"), getValue(t, trie, []byte("do")))
	value, err := trie.Get([]byte("dog"))
	require.Error(t, err)
	require.Empty(t, value)

	// Old
	trie.SetRootHash(rootHash)
	require.Equal(t, []byte("verb"), getValue(t, trie, []byte("do")))
	require.Equal(t, []byte("puppy"), getValue(t, trie, []byte("dog")))
}

func TestProof(t *testing.T) {
	t.Parallel()

	trie := CreateMerklePatriciaTrie(t)
	keys := [][]byte{[]byte("do"), []byte("dog"), []byte("doge"), []byte("horse")}
	values := [][]byte{[]byte("verb"), []byte("puppy"), []byte("coin"), []byte("stallion")}

	for i := range len(keys) {
		require.NoError(t, trie.Set(keys[i], values[i]))
	}

	// Check valid proofs
	for i, k := range keys {
		proof, err := trie.CreateProof(k)
		require.NoError(t, err)
		valFromProof, err := VerifyProof(proof, k)
		require.NoError(t, err)
		require.Equal(t, values[i], valFromProof)
	}

	// Check invalid proofs
	for _, k := range keys {
		proof, err := trie.CreateProof(k)
		require.NoError(t, err)
		valFromProof, err := VerifyProof(proof, []byte("wrong_key"))
		require.ErrorIs(t, err, db.ErrKeyNotFound)
		assert.Empty(t, valFromProof)
	}
}

func TestSmallRootHash(t *testing.T) {
	t.Parallel()

	holder := make(map[string][]byte)

	trie := NewMPT(NewMapSetter(holder), NewReader(NewMapGetter(holder)))
	key := []byte("key")
	value := []byte("value")

	require.NoError(t, trie.Set(key, value))
	assert.Equal(t, value, getValue(t, trie, key))

	trie2 := NewMPT(NewMapSetter(holder), NewReader(NewMapGetter(holder)))
	trie2.SetRootHash(trie.RootHash())

	assert.Equal(t, value, getValue(t, trie2, key))
}
