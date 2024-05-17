package mpt

import (
	"encoding/binary"
	"math/rand"
	"testing"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/core/db"
	"github.com/stretchr/testify/require"
)

func getValue(value interface{}, err error) interface{} {
	if err != nil {
		panic(err)
	}
	return value
}

func CreateMerklePatriciaTrie() *MerklePatriciaTrie {
	d, err := db.NewBadgerDbInMemory()
	if err != nil {
		panic("Failed to create BadgerDb")
	}
	trie := NewMerklePatriciaTrie(d, "mpt")
	return trie
}

func TestInsertGetOneShort(t *testing.T) {
	trie := CreateMerklePatriciaTrie()

	key := []byte("key")
	value := []byte("value")
	err := trie.Set(key, value)
	require.NoError(t, err)
	require.Equal(t, getValue(trie.Get(key)), value)

	gotValue, err := trie.Get([]byte("wrong_key"))
	require.Error(t, err)
	require.Equal(t, gotValue, []byte(nil))
}

func TestInsertGetOneLong(t *testing.T) {
	trie := CreateMerklePatriciaTrie()

	key := []byte("key_0000000000000000000000000000000000000000000000000000000000000000")
	value := []byte("value_0000000000000000000000000000000000000000000000000000000000000000")
	require.NoError(t, trie.Set(key, value))
	require.Equal(t, getValue(trie.Get(key)), value)
}

func TestInsertGetMany(t *testing.T) {
	trie := CreateMerklePatriciaTrie()

	require.NoError(t, trie.Set([]byte("do"), []byte("verb")))
	require.NoError(t, trie.Set([]byte("dog"), []byte("puppy")))
	require.NoError(t, trie.Set([]byte("doge"), []byte("coin")))
	require.NoError(t, trie.Set([]byte("horse"), []byte("stallion")))

	require.Equal(t, getValue(trie.Get([]byte("do"))), []byte("verb"))
	require.Equal(t, getValue(trie.Get([]byte("dog"))), []byte("puppy"))
	require.Equal(t, getValue(trie.Get([]byte("doge"))), []byte("coin"))
	require.Equal(t, getValue(trie.Get([]byte("horse"))), []byte("stallion"))
}

func TestIterate(t *testing.T) {
	trie := CreateMerklePatriciaTrie()
	keys := [][]byte{[]byte("do"), []byte("dog"), []byte("doge"), []byte("horse")}
	values := [][]byte{[]byte("verb"), []byte("puppy"), []byte("coin"), []byte("stallion")}

	for i := range len(keys) {
		require.NoError(t, trie.Set(keys[i], values[i]))
	}

	i := 0
	for kv := range trie.Iterate() {
		require.Equal(t, kv.Key, keys[i])
		require.Equal(t, kv.Value, values[i])
		i += 1
	}
	require.Len(t, keys, i)
}

func TestInsertGetLots(t *testing.T) {
	trie := CreateMerklePatriciaTrie()
	const size = 100

	var keys [size][]byte
	var values [size][]byte
	for i := 0; i < size; i++ {
		n := rand.Uint64()
		keys[i] = binary.LittleEndian.AppendUint64(keys[i], n)
		values[i] = binary.LittleEndian.AppendUint32(values[i], uint32(i))
	}

	for i, key := range keys {
		require.NoError(t, trie.Set(key, values[i]))
	}

	for i := range keys {
		require.Equal(t, getValue(trie.Get(keys[i])), values[i])
	}
}

func TestDeleteOne(t *testing.T) {
	trie := CreateMerklePatriciaTrie()

	require.NoError(t, trie.Set([]byte("key"), []byte("value")))
	require.NoError(t, trie.Delete([]byte("key")))

	value, err := trie.Get([]byte("key"))
	require.Equal(t, value, []byte(nil))
	require.Error(t, err)
}

func TestDeleteMany(t *testing.T) {
	trie := CreateMerklePatriciaTrie()

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
	trie := CreateMerklePatriciaTrie()
	const size = 100

	require.Equal(t, trie.RootHash(), common.EmptyHash)

	var keys [size][]byte
	var values [size][]byte
	for i := 0; i < size; i++ {
		keys[i] = binary.LittleEndian.AppendUint64(keys[i], rand.Uint64())
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
	trie := CreateMerklePatriciaTrie()

	require.NoError(t, trie.Set([]byte("do"), []byte("verb")))
	require.NoError(t, trie.Set([]byte("dog"), []byte("puppy")))

	rootHash := trie.RootHash()

	require.NoError(t, trie.Delete([]byte("dog")))
	require.NoError(t, trie.Set([]byte("do"), []byte("not_a_verb")))

	// Old
	trie2 := NewMerklePatriciaTrieWithRoot(trie.db, "mpt", rootHash)
	require.Equal(t, getValue(trie2.Get([]byte("do"))), []byte("verb"))
	require.Equal(t, getValue(trie2.Get([]byte("dog"))), []byte("puppy"))

	// New
	require.Equal(t, getValue(trie.Get([]byte("do"))), []byte("not_a_verb"))
	value, err := trie.Get([]byte("dog"))
	require.Equal(t, value, []byte(nil))
	require.Error(t, err)
}
