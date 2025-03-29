package mpt

import (
	"encoding/binary"
	rand "math/rand/v2"
	"testing"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type kvPair struct {
	key   []byte
	value []byte
}

func newRandGen() *rand.Rand {
	seed := [32]byte{10: 42, 20: 123}
	gen := rand.New(rand.NewChaCha8(seed)) //nolint:gosec
	return gen
}

func generateTestCase(gen *rand.Rand, numOps int, minKeyLen, maxKeyLen int, alphabet string) []kvPair {
	res := make([]kvPair, 0, numOps)

	for range numOps {
		keySize := minKeyLen + gen.IntN(maxKeyLen-minKeyLen+1)
		key := []byte{}
		for range keySize {
			char := alphabet[gen.IntN(len(alphabet))]
			key = append(key, char)
		}
		value := make([]byte, 8)
		binary.LittleEndian.PutUint64(value, gen.Uint64())
		res = append(res, kvPair{key, value})
	}

	return res
}

func getValue(t *testing.T, trie *MerklePatriciaTrie, key []byte) []byte {
	t.Helper()

	value, err := trie.Get(key)
	require.NoError(t, err)
	return value
}

func TestInsertGetOneShort(t *testing.T) {
	t.Parallel()

	trie := NewInMemMPT()
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

	trie := NewInMemMPT()

	key := []byte("key_0000000000000000000000000000000000000000000000000000000000000000")
	value := []byte("value_0000000000000000000000000000000000000000000000000000000000000000")
	require.NoError(t, trie.Set(key, value))
	require.Equal(t, value, getValue(t, trie, key))
}

func TestInsertGetMany(t *testing.T) {
	t.Parallel()

	trie := NewInMemMPT()

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

	trie := NewInMemMPT()
	// Check iteration on the empty trie
	for range trie.Iterate() {
	}

	keys := [][]byte{[]byte("do"), []byte("dog"), []byte("doge"), []byte("horse")}
	values := [][]byte{[]byte("verb"), []byte("puppy"), []byte("coin"), []byte("stallion")}

	for i := range keys {
		require.NoError(t, trie.Set(keys[i], values[i]))
	}

	i := 0
	for k, v := range trie.Iterate() {
		require.Equal(t, k, keys[i])
		require.Equal(t, v, values[i])
		i += 1
	}
	require.Len(t, keys, i)
}

func TestInsertGetLots(t *testing.T) {
	t.Parallel()

	trie := NewInMemMPT()
	const size uint32 = 100

	var keys [size][]byte
	var values [size][]byte
	for i := range size {
		n := rand.Uint64() //nolint:gosec
		keys[i] = binary.LittleEndian.AppendUint64(keys[i], n)
		values[i] = binary.LittleEndian.AppendUint32(values[i], i)
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

	trie := NewInMemMPT()

	require.NoError(t, trie.Set([]byte("key"), []byte("value")))
	require.NoError(t, trie.Delete([]byte("key")))

	value, err := trie.Get([]byte("key"))
	require.Equal(t, value, []byte(nil))
	require.Error(t, err)
}

func TestDeleteMany(t *testing.T) {
	t.Parallel()

	trie := NewInMemMPT()

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

	trie := NewInMemMPT()
	const size uint32 = 100

	require.Equal(t, trie.RootHash(), common.EmptyHash)

	var keys [size][]byte
	var values [size][]byte
	for i := range size {
		keys[i] = binary.LittleEndian.AppendUint64(keys[i], rand.Uint64()) //nolint:gosec
		values[i] = binary.LittleEndian.AppendUint32(values[i], i)
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

	trie := NewInMemMPT()

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

func TestSmallRootHash(t *testing.T) {
	t.Parallel()

	holder := NewInMemHolder()

	trie := NewMPTFromMap(holder)
	key := []byte("key")
	value := []byte("value")

	require.NoError(t, trie.Set(key, value))
	assert.Equal(t, value, getValue(t, trie, key))

	trie2 := NewMPTFromMap(holder)
	trie2.SetRootHash(trie.RootHash())

	assert.Equal(t, value, getValue(t, trie2, key))
}

func TestInsertBatch(t *testing.T) {
	t.Parallel()

	gen := newRandGen()
	// pick short alphabet and short keys to increase key collisions count
	treeOps := generateTestCase(gen, 1000, 1, 8, "abcdefgh")

	trie := NewInMemMPT()
	trieBatch := NewInMemMPT()

	checkTreesEqual := func(t *testing.T) {
		t.Helper()

		data := make(map[string]uint64)
		dataBatch := make(map[string]uint64)

		for k, v := range trie.Iterate() {
			data[string(k)] = binary.LittleEndian.Uint64(v)
		}
		for k, v := range trieBatch.Iterate() {
			dataBatch[string(k)] = binary.LittleEndian.Uint64(v)
		}

		require.Equal(t, data, dataBatch)
		require.Equal(t, trie.RootHash(), trieBatch.RootHash())
	}

	keys := make([][]byte, 0, len(treeOps))
	values := make([][]byte, 0, len(treeOps))

	for _, kv := range treeOps {
		require.NoError(t, trie.Set(kv.key, kv.value))

		keys = append(keys, kv.key)
		values = append(values, kv.value)
	}
	require.NoError(t, trieBatch.SetBatch(keys, values))

	checkTreesEqual(t)

	// increase each value by 1
	for i, kv := range treeOps {
		v := binary.LittleEndian.Uint64(kv.value) + 1
		binary.LittleEndian.PutUint64(values[i], v)
		require.NoError(t, trie.Set(kv.key, values[i]))
	}
	require.NoError(t, trieBatch.SetBatch(keys, values))

	checkTreesEqual(t)

	// now generate some new insertions
	treeOps = generateTestCase(gen, 1000, 1, 8, "abcdefgh")
	for i, kv := range treeOps {
		require.NoError(t, trie.Set(kv.key, kv.value))

		keys[i] = kv.key
		values[i] = kv.value
	}
	require.NoError(t, trieBatch.SetBatch(keys, values))

	checkTreesEqual(t)
}

func BenchmarkTreeInsertions(b *testing.B) {
	b.Run("simple insert", func(b *testing.B) {
		trie := NewInMemMPT()
		gen := newRandGen()

		// long keys and alphabet to increase key uniqueness
		treeOps := generateTestCase(gen, b.N, 5, 32, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
		for _, kv := range treeOps {
			require.NoError(b, trie.Set(kv.key, kv.value))
		}

		// update each key with new value
		for _, kv := range treeOps {
			value := binary.LittleEndian.Uint64(kv.value)
			binary.LittleEndian.PutUint64(kv.value, value+1)
			require.NoError(b, trie.Set(kv.key, kv.value))
		}

		// and insert some new keys
		for _, kv := range generateTestCase(
			gen,
			b.N,
			2,
			16,
			"abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789",
		) {
			require.NoError(b, trie.Set(kv.key, kv.value))
		}
	})

	b.Run("batch insert", func(b *testing.B) {
		trie := NewInMemMPT()
		gen := newRandGen()

		// long keys and alphabet to increase key uniqueness
		treeOps := generateTestCase(gen, b.N, 5, 32, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

		keys := make([][]byte, 0, len(treeOps))
		values := make([][]byte, 0, len(treeOps))

		uniq := map[string]struct{}{}
		for _, kv := range treeOps {
			uniq[string(kv.key)] = struct{}{}
			keys = append(keys, kv.key)
			values = append(values, kv.value)
		}
		require.Greater(b, len(uniq), 90.0*len(keys)/100)

		require.NoError(b, trie.SetBatch(keys, values))

		// update each key with new value
		for i := range values {
			v := binary.LittleEndian.Uint64(values[i]) + 1
			binary.LittleEndian.PutUint64(values[i], v)
		}
		require.NoError(b, trie.SetBatch(keys, values))

		// and insert some new keys
		for i, kv := range generateTestCase(
			gen,
			b.N,
			2,
			16,
			"abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789",
		) {
			keys[i] = kv.key
			values[i] = kv.value
		}
		require.NoError(b, trie.SetBatch(keys, values))
	})
}
