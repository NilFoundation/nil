package db

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/NilFoundation/nil/nil/tests/detectrace"
	"github.com/dgraph-io/badger/v4"
	ds "github.com/ipfs/go-datastore"
	dsq "github.com/ipfs/go-datastore/query"
	dstest "github.com/ipfs/go-datastore/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testcases = map[string]string{
	"/a":     "a",
	"/a/b":   "ab",
	"/a/b/c": "abc",
	"/a/b/d": "a/b/d",
	"/a/c":   "ac",
	"/a/d":   "ad",
	"/e":     "e",
	"/f":     "f",
	"/g":     "",
}

// returns datastore, and a function to call on exit.
// (this garbage collects). So:
//
//	d, close := newDS(t, nil)
//	defer close()
func newDS(t *testing.T, opts *Options) (*Datastore, func()) {
	t.Helper()

	path := t.TempDir()

	if opts == nil {
		opts = &DefaultOptions
	}

	bOpts := badger.DefaultOptions(path)
	// Limit memory usage for tests, mostly because they fail on 32-bit systems.
	bOpts.ValueLogFileSize = 104857600 // 100 MiB as we have problems running tests on 32bit
	bOpts.MemTableSize = 41943040      // 40 MiB
	bOpts.NumMemtables = 1

	db, err := badger.Open(bOpts)
	require.NoError(t, err)

	d := NewDatastore(db, TableName(t.Name()), opts)
	return d, func() {
		require.NoError(t, d.Close())
		os.RemoveAll(path)
	}
}

func addTestCases(t *testing.T, d *Datastore, testcases map[string]string) {
	t.Helper()

	for k, v := range testcases {
		dsk := ds.NewKey(k)
		if err := d.Put(t.Context(), dsk, []byte(v)); err != nil {
			t.Fatal(err)
		}
	}

	for k, v := range testcases {
		dsk := ds.NewKey(k)
		v2, err := d.Get(t.Context(), dsk)
		if err != nil {
			t.Fatal(err)
		}
		if string(v2) != v {
			t.Errorf("%s values differ: %s != %s", k, v, v2)
		}
	}
}

func TestQuery(t *testing.T) {
	t.Parallel()

	d, done := newDS(t, nil)
	defer done()

	addTestCases(t, d, testcases)

	rs, err := d.Query(t.Context(), dsq.Query{Prefix: "/a/"})
	require.NoError(t, err)

	expectMatches(t, []string{
		"/a/b",
		"/a/b/c",
		"/a/b/d",
		"/a/c",
		"/a/d",
	}, rs)

	// test offset and limit

	rs, err = d.Query(t.Context(), dsq.Query{Prefix: "/a/", Offset: 2, Limit: 2})
	require.NoError(t, err)

	expectMatches(t, []string{
		"/a/b/d",
		"/a/c",
	}, rs)
}

func TestHas(t *testing.T) {
	t.Parallel()

	d, done := newDS(t, nil)
	defer done()
	addTestCases(t, d, testcases)

	has, err := d.Has(t.Context(), ds.NewKey("/a/b/c"))
	require.NoError(t, err)

	if !has {
		t.Error("Key should be found")
	}

	has, err = d.Has(t.Context(), ds.NewKey("/a/b/c/d"))
	require.NoError(t, err)

	if has {
		t.Error("Key should not be found")
	}
}

func TestGetSize(t *testing.T) {
	t.Parallel()

	d, done := newDS(t, nil)
	defer done()
	addTestCases(t, d, testcases)

	size, err := d.GetSize(t.Context(), ds.NewKey("/a/b/c"))
	if err != nil {
		t.Error(err)
	}

	if size != len(testcases["/a/b/c"]) {
		t.Error("")
	}

	_, err = d.GetSize(t.Context(), ds.NewKey("/a/b/c/d"))
	require.ErrorIs(t, err, ds.ErrNotFound)
}

func TestNotExistGet(t *testing.T) {
	t.Parallel()

	d, done := newDS(t, nil)
	defer done()
	addTestCases(t, d, testcases)

	has, err := d.Has(t.Context(), ds.NewKey("/a/b/c/d"))
	if err != nil {
		t.Error(err)
	}

	if has {
		t.Error("Key should not be found")
	}

	val, err := d.Get(t.Context(), ds.NewKey("/a/b/c/d"))
	if val != nil {
		t.Error("Key should not be found")
	}
	require.ErrorIs(t, err, ds.ErrNotFound)
}

func TestDelete(t *testing.T) {
	t.Parallel()

	d, done := newDS(t, nil)
	defer done()
	addTestCases(t, d, testcases)

	has, err := d.Has(t.Context(), ds.NewKey("/a/b/c"))
	if err != nil {
		t.Error(err)
	}
	if !has {
		t.Error("Key should be found")
	}

	err = d.Delete(t.Context(), ds.NewKey("/a/b/c"))
	if err != nil {
		t.Error(err)
	}

	has, err = d.Has(t.Context(), ds.NewKey("/a/b/c"))
	require.NoError(t, err)
	if has {
		t.Error("Key should not be found")
	}
}

func TestGetEmpty(t *testing.T) {
	t.Parallel()

	d, done := newDS(t, nil)
	defer done()

	err := d.Put(t.Context(), ds.NewKey("/a"), []byte{})
	require.NoError(t, err)

	v, err := d.Get(t.Context(), ds.NewKey("/a"))
	require.NoError(t, err)

	if len(v) != 0 {
		t.Error("expected 0 len []byte form get")
	}
}

func expectMatches(t *testing.T, expect []string, actualR dsq.Results) {
	t.Helper()

	actual, err := actualR.Rest()
	require.NoError(t, err)

	if len(actual) != len(expect) {
		t.Error("not enough", expect, actual)
	}
	for _, k := range expect {
		found := false
		for _, e := range actual {
			if e.Key == k {
				found = true
			}
		}
		if !found {
			t.Error(k, "not found")
		}
	}
}

func TestBatching(t *testing.T) {
	t.Parallel()

	d, done := newDS(t, nil)
	defer done()

	b, err := d.Batch(t.Context())
	require.NoError(t, err)

	for k, v := range testcases {
		err := b.Put(t.Context(), ds.NewKey(k), []byte(v))
		require.NoError(t, err)
	}

	err = b.Commit(t.Context())
	require.NoError(t, err)

	for k, v := range testcases {
		val, err := d.Get(t.Context(), ds.NewKey(k))
		require.NoError(t, err)

		if v != string(val) {
			t.Fatal("got wrong data!")
		}
	}

	// Test delete

	b, err = d.Batch(t.Context())
	require.NoError(t, err)

	err = b.Delete(t.Context(), ds.NewKey("/a/b"))
	require.NoError(t, err)

	err = b.Delete(t.Context(), ds.NewKey("/a/b/c"))
	require.NoError(t, err)

	err = b.Commit(t.Context())
	require.NoError(t, err)

	rs, err := d.Query(t.Context(), dsq.Query{Prefix: "/"})
	require.NoError(t, err)

	expectMatches(t, []string{
		"/a",
		"/a/b/d",
		"/a/c",
		"/a/d",
		"/e",
		"/f",
		"/g",
	}, rs)

	// Test cancel

	b, err = d.Batch(t.Context())
	require.NoError(t, err)

	const key = "/xyz"

	err = b.Put(t.Context(), ds.NewKey(key), []byte("/x/y/z"))
	require.NoError(t, err)

	// TODO: remove type assertion once datastore.Batch interface has Cancel
	batch, ok := b.(*batch)
	require.True(t, ok)
	err = batch.Cancel()
	require.NoError(t, err)

	_, err = d.Get(t.Context(), ds.NewKey(key))
	require.Error(t, err)

	// Test with TTL

	opts := DefaultOptions.WithTTL(time.Second)
	d, done = newDS(t, &opts)
	defer done()

	b, err = d.Batch(t.Context())
	require.NoError(t, err)

	for k, v := range testcases {
		err := b.Put(t.Context(), ds.NewKey(k), []byte(v))
		if err != nil {
			t.Fatal(err)
		}
	}

	err = b.Commit(t.Context())
	require.NoError(t, err)

	// check data was set correctly
	for k, v := range testcases {
		val, err := d.Get(t.Context(), ds.NewKey(k))
		require.NoError(t, err)

		if v != string(val) {
			t.Fatal("got wrong data!")
		}
	}

	time.Sleep(time.Second)

	// check data has expired
	for k := range testcases {
		has, err := d.Has(t.Context(), ds.NewKey(k))
		require.NoError(t, err)
		if has {
			t.Fatal("record with ttl did not expire")
		}
	}
}

func TestBatchingRequired(t *testing.T) {
	t.Parallel()

	path := t.TempDir()

	bOpts := badger.DefaultOptions(path)
	bOpts.ValueLogFileSize = 104857600 // 100 MiB as we have problems running tests on 32bit
	bOpts.MemTableSize = 41943040      // 40 MiB
	bOpts.NumMemtables = 1

	db, err := badger.Open(bOpts)
	require.NoError(t, err)

	d := NewDatastore(db, TableName(t.Name()), nil)
	defer func() {
		require.NoError(t, d.Close())
		os.RemoveAll(path)
	}()

	const valSize = 1000

	// Check that transaction fails when there are too many writes. This is
	// not testing batching logic, but is here to prove that batching works
	// where a transaction fails.
	t.Logf("putting %d byte values until transaction overflows", valSize)
	tx, err := d.NewTransaction(t.Context(), false)
	require.NoError(t, err)

	var puts int
	for ; puts < 10000000; puts++ {
		buf := make([]byte, valSize)
		_, randErr := rand.Read(buf)
		require.NoError(t, randErr)
		err = tx.Put(t.Context(), ds.NewKey(fmt.Sprintf("/key%d", puts)), buf)
		if err != nil {
			break
		}
		puts++
	}
	require.Errorf(t, err, "transaction cannot handle %d puts", puts)
	tx.Discard(t.Context())

	// Check that batch succeeds with the same number of writes that caused a
	// transaction to fail.
	t.Logf("putting %d %d byte values using batch", puts, valSize)
	b, err := d.Batch(t.Context())
	require.NoError(t, err)
	for i := range puts {
		buf := make([]byte, valSize)
		_, err := rand.Read(buf)
		require.NoError(t, err)
		err = b.Put(t.Context(), ds.NewKey(fmt.Sprintf("/key%d", i)), buf)
		require.NoError(t, err)
	}

	err = b.Commit(t.Context())
	require.NoError(t, err)
}

// Tests from basic_tests from go-datastore

func TestBasicPutGet(t *testing.T) {
	t.Parallel()

	d, done := newDS(t, nil)
	defer done()

	k := ds.NewKey("foo")
	val := []byte("Hello Datastore!")

	err := d.Put(t.Context(), k, val)
	require.NoError(t, err, "error putting to datastore")

	have, err := d.Has(t.Context(), k)
	require.NoError(t, err, "error calling has on key we just put")

	if !have {
		t.Fatal("should have key foo, has returned false")
	}

	out, err := d.Get(t.Context(), k)
	require.NoError(t, err)

	if !bytes.Equal(out, val) {
		t.Fatal("value received on get wasn't what we expected:", out)
	}

	have, err = d.Has(t.Context(), k)
	require.NoError(t, err)

	if !have {
		t.Fatal("should have key foo, has returned false")
	}

	err = d.Delete(t.Context(), k)
	require.NoError(t, err)

	have, err = d.Has(t.Context(), k)
	require.NoError(t, err)

	if have {
		t.Fatal("should not have key foo, has returned true")
	}
}

func TestNotFounds(t *testing.T) {
	t.Parallel()

	d, done := newDS(t, nil)
	defer done()

	badk := ds.NewKey("notreal")

	val, err := d.Get(t.Context(), badk)
	require.ErrorIs(t, err, ds.ErrNotFound)

	if val != nil {
		t.Fatal("get should always return nil for not found values")
	}

	have, err := d.Has(t.Context(), badk)
	require.NoError(t, err)
	if have {
		t.Fatal("has returned true for key we don't have")
	}
}

func TestManyKeysAndQuery(t *testing.T) {
	t.Parallel()

	d, done := newDS(t, nil)
	defer done()

	count := 100
	keys := make([]ds.Key, 0, count)
	keystrs := make([]string, 0, count)
	values := make([][]byte, 0, count)
	for i := range count {
		s := fmt.Sprintf("%dkey%d", i, i)
		dsk := ds.NewKey(s)
		keystrs = append(keystrs, dsk.String())
		keys = append(keys, dsk)
		buf := make([]byte, 64)
		_, err := rand.Read(buf)
		require.NoError(t, err)
		values = append(values, buf)
	}

	t.Logf("putting %d values", count)
	for i, k := range keys {
		err := d.Put(t.Context(), k, values[i])
		require.NoError(t, err)
	}

	t.Log("getting values back")
	for i, k := range keys {
		val, err := d.Get(t.Context(), k)
		require.NoError(t, err)

		if !bytes.Equal(val, values[i]) {
			t.Fatal("input value didn't match the one returned from Get")
		}
	}

	t.Log("querying values")
	q := dsq.Query{KeysOnly: true}
	resp, err := d.Query(t.Context(), q)
	require.NoError(t, err)

	t.Log("aggregating query results")
	var outkeys []string
	for {
		res, ok := resp.NextSync()
		require.NoError(t, res.Error)
		if !ok {
			break
		}

		outkeys = append(outkeys, res.Key)
	}

	t.Log("verifying query output")
	sort.Strings(keystrs)
	sort.Strings(outkeys)

	if len(keystrs) != len(outkeys) {
		t.Fatalf("got wrong number of keys back, %d != %d", len(keystrs), len(outkeys))
	}

	for i, s := range keystrs {
		if outkeys[i] != s {
			t.Fatalf("in key output, got %s but expected %s", outkeys[i], s)
		}
	}

	t.Log("deleting all keys")
	for _, k := range keys {
		require.NoError(t, d.Delete(t.Context(), k))
	}
}

func TestGC(t *testing.T) {
	t.Parallel()

	d, done := newDS(t, nil)
	defer done()

	count := 10000

	b, err := d.Batch(t.Context())
	require.NoError(t, err)

	t.Logf("putting %d values", count)
	for i := range count {
		buf := make([]byte, 6400)
		_, err := rand.Read(buf)
		require.NoError(t, err)
		err = b.Put(t.Context(), ds.NewKey(fmt.Sprintf("/key%d", i)), buf)
		if err != nil {
			t.Fatal(err)
		}
	}

	err = b.Commit(t.Context())
	require.NoError(t, err)

	b, err = d.Batch(t.Context())
	require.NoError(t, err)

	t.Logf("deleting %d values", count)
	for i := range count {
		err := b.Delete(t.Context(), ds.NewKey(fmt.Sprintf("/key%d", i)))
		require.NoError(t, err)
	}

	err = b.Commit(t.Context())
	require.NoError(t, err)

	if err := d.CollectGarbage(t.Context()); err != nil {
		t.Fatal(err)
	}
}

// TestDisksage verifies we fetch some badger size correctly.
// Because the Size metric is only updated every minute in badger and
// this interval is not configurable, we re-open the database
// (the size is always calculated on Open) to make things quick.
func TestDiskUsage(t *testing.T) {
	t.Parallel()

	path := t.TempDir()
	defer os.RemoveAll(path)

	opts := badger.DefaultOptions(path)
	opts.ValueLogFileSize = 104857600 // 100 MiB as we have problems running tests on 32bit
	opts.MemTableSize = 41943040      // 40 MiB
	opts.NumMemtables = 1

	db, err := badger.Open(opts)
	require.NoError(t, err)

	d := NewDatastore(db, TableName(t.Name()), nil)

	addTestCases(t, d, testcases)
	require.NoError(t, d.Close())

	db, err = badger.Open(opts)
	require.NoError(t, err)

	d = NewDatastore(db, TableName(t.Name()), nil)

	s, _ := d.DiskUsage(t.Context())
	if s == 0 {
		t.Error("expected some size")
	}
	require.NoError(t, d.Close())
}

func TestTxnDiscard(t *testing.T) {
	t.Parallel()

	path := t.TempDir()
	defer os.RemoveAll(path)

	opts := badger.DefaultOptions(path)
	opts.ValueLogFileSize = 104857600 // 100 MiB as we have problems running tests on 32bit
	opts.MemTableSize = 41943040      // 40 MiB
	opts.NumMemtables = 1

	db, err := badger.Open(opts)
	require.NoError(t, err)

	d := NewDatastore(db, TableName(t.Name()), nil)
	txn, err := d.NewTransaction(t.Context(), false)
	require.NoError(t, err)
	key := ds.NewKey("/test/thing")
	if err := txn.Put(t.Context(), key, []byte{1, 2, 3}); err != nil {
		t.Fatal(err)
	}
	txn.Discard(t.Context())
	has, err := d.Has(t.Context(), key)
	require.NoError(t, err)
	if has {
		t.Fatal("key written in aborted transaction still exists")
	}

	require.NoError(t, d.Close())
}

func TestTxnCommit(t *testing.T) {
	t.Parallel()

	path := t.TempDir()
	defer os.RemoveAll(path)

	opts := badger.DefaultOptions(path)
	opts.ValueLogFileSize = 104857600 // 100 MiB as we have problems running tests on 32bit
	opts.MemTableSize = 41943040      // 40 MiB
	opts.NumMemtables = 1

	db, err := badger.Open(opts)
	require.NoError(t, err)

	d := NewDatastore(db, TableName(t.Name()), nil)

	txn, err := d.NewTransaction(t.Context(), false)
	require.NoError(t, err)

	key := ds.NewKey("/test/thing")
	require.NoError(t, txn.Put(t.Context(), key, []byte{1, 2, 3}))

	err = txn.Commit(t.Context())
	require.NoError(t, err)

	has, err := d.Has(t.Context(), key)
	require.NoError(t, err)
	if !has {
		t.Fatal("key written in committed transaction does not exist")
	}

	require.NoError(t, d.Close())
}

func TestTxnBatch(t *testing.T) {
	t.Parallel()

	path := t.TempDir()
	defer os.RemoveAll(path)

	opts := badger.DefaultOptions(path)
	opts.ValueLogFileSize = 104857600 // 100 MiB as we have problems running tests on 32bit
	opts.MemTableSize = 41943040      // 40 MiB
	opts.NumMemtables = 1

	db, err := badger.Open(opts)
	require.NoError(t, err)

	d := NewDatastore(db, TableName(t.Name()), nil)

	txn, err := d.NewTransaction(t.Context(), false)
	require.NoError(t, err)
	data := make(map[ds.Key][]byte)
	for i := range 10 {
		key := ds.NewKey(fmt.Sprintf("/test/%d", i))
		bytes := make([]byte, 16)
		_, err := rand.Read(bytes)
		require.NoError(t, err)
		data[key] = bytes

		err = txn.Put(t.Context(), key, bytes)
		if err != nil {
			t.Fatal(err)
		}
	}
	err = txn.Commit(t.Context())
	require.NoError(t, err)

	for key, bytes := range data {
		retrieved, err := d.Get(t.Context(), key)
		require.NoError(t, err)
		if len(retrieved) != len(bytes) {
			t.Fatal("bytes stored different length from bytes generated")
		}
		for i, b := range retrieved {
			if bytes[i] != b {
				t.Fatal("bytes stored different content from bytes generated")
			}
		}
	}

	require.NoError(t, d.Close())
}

func TestTTL(t *testing.T) {
	t.Parallel()

	if detectrace.WithRace() {
		t.Skip("disabling timing dependent test while race detector is enabled")
	}

	path := t.TempDir()
	defer os.RemoveAll(path)

	opts := badger.DefaultOptions(path)
	opts.ValueLogFileSize = 104857600 // 100 MiB as we have problems running tests on 32bit
	opts.MemTableSize = 41943040      // 40 MiB
	opts.NumMemtables = 1

	db, err := badger.Open(opts)
	require.NoError(t, err)

	d := NewDatastore(db, TableName(t.Name()), nil)

	txn, err := d.NewTransaction(t.Context(), false)
	require.NoError(t, err)

	data := make(map[ds.Key][]byte)
	for i := range 10 {
		key := ds.NewKey(fmt.Sprintf("/test/%d", i))
		bytes := make([]byte, 16)
		_, err := rand.Read(bytes)
		require.NoError(t, err)
		data[key] = bytes
	}

	// write data
	for key, bytes := range data {
		dsTTL, ok := txn.(ds.TTL)
		require.True(t, ok)
		require.NoError(t, dsTTL.PutWithTTL(t.Context(), key, bytes, time.Second))
	}
	err = txn.Commit(t.Context())
	require.NoError(t, err)

	// set ttl
	txn, err = d.NewTransaction(t.Context(), false)
	require.NoError(t, err)
	for key := range data {
		dsTTL, ok := txn.(ds.TTL)
		require.True(t, ok)
		require.NoError(t, dsTTL.SetTTL(t.Context(), key, time.Second))
	}
	err = txn.Commit(t.Context())
	require.NoError(t, err)

	txn, err = d.NewTransaction(t.Context(), true)
	require.NoError(t, err)
	for key := range data {
		_, err := txn.Get(t.Context(), key)
		require.NoError(t, err)
	}
	txn.Discard(t.Context())

	time.Sleep(time.Second)

	for key := range data {
		has, err := d.Has(t.Context(), key)
		require.NoError(t, err)
		if has {
			t.Fatal("record with ttl did not expire")
		}
	}

	require.NoError(t, d.Close())
}

func TestExpirations(t *testing.T) {
	t.Parallel()

	var err error

	d, done := newDS(t, nil)
	defer done()

	txn, err := d.NewTransaction(t.Context(), false)
	require.NoError(t, err)
	ttltxn, ok := txn.(ds.TTL)
	require.True(t, ok)
	defer txn.Discard(t.Context())

	key := ds.NewKey("/abc/def")
	val := make([]byte, 32)
	n, err := rand.Read(val)
	require.NoError(t, err)
	require.Equal(t, 32, n)

	ttl := time.Hour
	now := time.Now()
	tgt := now.Add(ttl)

	require.NoError(t, ttltxn.PutWithTTL(t.Context(), key, val, ttl))
	require.NoError(t, txn.Commit(t.Context()))

	// Second transaction to retrieve expirations.
	txn, err = d.NewTransaction(t.Context(), true)
	require.NoError(t, err)
	ttltxn, ok = txn.(ds.TTL)
	require.True(t, ok)
	defer txn.Discard(t.Context())

	// GetExpiration returns expected value.
	var dsExp time.Time
	if dsExp, err = ttltxn.GetExpiration(t.Context(), key); err != nil {
		t.Fatalf("getting expiration failed: %v", err)
	} else if tgt.Sub(dsExp) >= 5*time.Second {
		t.Fatal("expiration returned by datastore not within the expected range (tolerance: 5 seconds)")
	} else if tgt.Sub(dsExp) < 0 {
		t.Fatal("expiration returned by datastore was earlier than expected")
	}

	// Iterator returns expected value.
	q := dsq.Query{
		ReturnExpirations: true,
		KeysOnly:          true,
	}
	var ress dsq.Results
	if ress, err = txn.Query(t.Context(), q); err != nil {
		t.Fatalf("querying datastore failed: %v", err)
	}

	defer ress.Close()
	if res, ok := ress.NextSync(); !ok {
		t.Fatal("expected 1 result in iterator")
	} else if res.Expiration != dsExp {
		t.Fatalf(
			"expiration returned from iterator differs from GetExpiration, expected: %v, actual: %v",
			dsExp,
			res.Expiration)
	}

	if _, ok := ress.NextSync(); ok {
		t.Fatal("expected no more results in iterator")
	}

	// Datastore->GetExpiration()
	if exp, err := d.GetExpiration(t.Context(), key); err != nil {
		t.Fatalf("querying datastore failed: %v", err)
	} else if exp != dsExp {
		t.Fatalf("expiration returned from DB differs from that returned by txn, expected: %v, actual: %v", dsExp, exp)
	}

	_, err = d.GetExpiration(t.Context(), ds.NewKey("/foo/bar"))
	require.ErrorIs(t, err, ds.ErrNotFound)
}

func TestOptions(t *testing.T) {
	t.Parallel()

	path := t.TempDir()
	opts := DefaultOptions
	opts.TTL = time.Minute

	bOpts := badger.DefaultOptions(path)
	bOpts.ValueLogFileSize = 104857600 // 100 MiB as we have problems running tests on 32bit
	bOpts.MemTableSize = 41943040      // 40 MiB
	bOpts.NumMemtables = 1

	db, err := badger.Open(bOpts)
	require.NoError(t, err)

	d := NewDatastore(db, TableName(t.Name()), &opts)
	if d.ttl != time.Minute {
		t.Fatal("datastore ttl not set")
	}

	ratio := 0.5
	ttl := 4 * time.Second
	o := DefaultOptions.
		WithTTL(ttl).
		WithGcDiscardRatio(ratio)

	assert.Equal(t, ttl, o.TTL)
	assert.InEpsilon(t, ratio, o.GcDiscardRatio, 0.01)

	// Make sure DefaultOptions aren't changed
	assert.Equal(t, time.Duration(0), DefaultOptions.TTL)
}

func TestClosedError(t *testing.T) {
	t.Parallel()

	path := t.TempDir()
	opts := DefaultOptions

	bOpts := badger.DefaultOptions(path)
	bOpts.ValueLogFileSize = 104857600 // 100 MiB as we have problems running tests on 32bit
	bOpts.MemTableSize = 41943040      // 40 MiB
	bOpts.NumMemtables = 1

	db, err := badger.Open(bOpts)
	require.NoError(t, err)

	d := NewDatastore(db, TableName(t.Name()), &opts)
	dstx, err := d.NewTransaction(t.Context(), false)
	require.NoError(t, err)

	tx, ok := dstx.(*txn)
	require.True(t, ok)

	require.NoError(t, d.Close())
	os.RemoveAll(path)

	key := ds.NewKey("/a/b/c")

	_, err = d.NewTransaction(t.Context(), false)
	require.ErrorIs(t, err, ErrClosed)

	require.ErrorIs(t, d.Put(t.Context(), key, nil), ErrClosed)
	require.ErrorIs(t, d.Sync(t.Context(), key), ErrClosed)
	require.ErrorIs(t, d.PutWithTTL(t.Context(), key, nil, time.Second), ErrClosed)
	require.ErrorIs(t, d.Close(), ErrClosed)
	require.ErrorIs(t, d.CollectGarbage(t.Context()), ErrClosed)
	require.ErrorIs(t, d.gcOnce(), ErrClosed)
	require.ErrorIs(t, tx.Put(t.Context(), key, nil), ErrClosed)
	require.ErrorIs(t, tx.Sync(t.Context(), key), ErrClosed)
	require.ErrorIs(t, tx.PutWithTTL(t.Context(), key, nil, time.Second), ErrClosed)
	require.ErrorIs(t, tx.SetTTL(t.Context(), key, time.Second), ErrClosed)
	require.ErrorIs(t, tx.Commit(t.Context()), ErrClosed)
	require.ErrorIs(t, tx.Close(), ErrClosed)
	require.ErrorIs(t, tx.Delete(t.Context(), key), ErrClosed)

	err = d.SetTTL(t.Context(), key, time.Second)
	require.ErrorIs(t, err, ErrClosed)

	_, err = d.GetExpiration(t.Context(), ds.Key{})
	require.ErrorIs(t, err, ErrClosed)

	_, err = d.Get(t.Context(), key)
	require.ErrorIs(t, err, ErrClosed)

	_, err = d.Has(t.Context(), key)
	require.ErrorIs(t, err, ErrClosed)

	_, err = d.GetSize(t.Context(), key)
	require.ErrorIs(t, err, ErrClosed)

	_, err = d.Query(t.Context(), dsq.Query{})
	require.ErrorIs(t, err, ErrClosed)

	_, err = d.DiskUsage(t.Context())
	require.ErrorIs(t, err, ErrClosed)

	_, err = tx.GetExpiration(t.Context(), key)
	require.ErrorIs(t, err, ErrClosed)

	_, err = tx.Get(t.Context(), key)
	require.ErrorIs(t, err, ErrClosed)

	_, err = tx.Has(t.Context(), key)
	require.ErrorIs(t, err, ErrClosed)

	_, err = tx.GetSize(t.Context(), key)
	require.ErrorIs(t, err, ErrClosed)

	_, err = tx.Query(t.Context(), dsq.Query{})
	require.ErrorIs(t, err, ErrClosed)
}

func TestDefaultTTL(t *testing.T) {
	t.Parallel()

	opts := DefaultOptions.WithTTL(time.Second)
	d, done := newDS(t, &opts)
	defer done()

	data1 := make(map[ds.Key][]byte)
	data2 := make(map[ds.Key][]byte)
	for i := range 10 {
		key1 := ds.NewKey(fmt.Sprintf("/test1/%d", i))
		key2 := ds.NewKey(fmt.Sprintf("/test2/%d", i))
		bytes := make([]byte, 16)
		_, err := rand.Read(bytes)
		require.NoError(t, err)
		data1[key1] = bytes
		data2[key2] = bytes
	}

	// put directly into datastore
	for key, bytes := range data1 {
		err := d.Put(t.Context(), key, bytes)
		require.NoError(t, err)

		// check data was persisted
		has, err := d.Has(t.Context(), key)
		require.NoError(t, err)
		assert.True(t, has, "record not in db")
	}

	// put via transactions
	for key, bytes := range data2 {
		tx, err := d.NewTransaction(t.Context(), false)
		require.NoError(t, err)

		err = tx.Put(t.Context(), key, bytes)
		require.NoError(t, err)

		err = tx.Commit(t.Context())
		require.NoError(t, err)

		// check data was persisted
		has, err := d.Has(t.Context(), key)
		require.NoError(t, err)
		assert.True(t, has, "record not in db")
	}

	time.Sleep(time.Second)

	// check datastore data has expired
	for key := range data1 {
		has, err := d.Has(t.Context(), key)
		require.NoError(t, err)
		assert.False(t, has, "record with ttl did not expire")
	}

	// check txn data has expired
	for key := range data2 {
		has, err := d.Has(t.Context(), key)
		require.NoError(t, err)
		assert.False(t, has, "record with ttl did not expire")
	}
}

func TestSuite(t *testing.T) {
	t.Parallel()

	d, done := newDS(t, nil)
	defer done()

	dstest.SubtestAll(t, d)
}
