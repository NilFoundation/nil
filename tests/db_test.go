package tests

import (
	"bytes"
	"context"
	db "github.com/NilFoundation/nil/core/db"
	"testing"
)

func TestTransaction(t *testing.T) {
	db := db.NewSqlite(t.TempDir() + "/foo.db")
	defer db.Close()

	ctx := context.Background()

	tx, err := db.BeginTx(ctx)

	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()

	tx2, err := db.BeginTx(ctx)

	if err != nil {
		t.Fatal(err)
	}

	defer tx2.Rollback()

	err = tx.Put("tbl", []byte("foo"), []byte("bar"))

	if err != nil {
		t.Fatal(err)
	}

	val, err := tx.GetOne("tbl", []byte("foo"))

	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(val, []byte("bar")) {
		t.Fatal("Values not equal: ", val)
	}

	has, err := tx.Has("tbl", []byte("foo"))

	if err != nil {
		t.Fatal(err)
	}

	if has == false {
		t.Fatal("Key 'foo' should be present")
	}

	// Testing that parallel transactions don't see changes made by the first one

	has, err = tx2.Has("tbl", []byte("foo"))

	if err != nil {
		t.Fatal(err)
	}

	if has == true {
		t.Fatal("Key 'foo' should not be present from the second transaction")
	}

	tx2.Rollback()

	err = tx.Commit()
	if err != nil {
		t.Fatal(err)
	}

	// Testing deletion of rows

	tx, err = db.BeginTx(ctx)

	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()

	has, err = tx.Has("tbl", []byte("foo"))

	if err != nil {
		t.Fatal(err)
	}

	if has == false {
		t.Fatal("Key 'foo' should be present")
	}

	err = tx.Delete("tbl", []byte("foo"))

	if err != nil {
		t.Fatal(err)
	}

	has, err = tx.Has("tbl", []byte("foo"))

	if err != nil {
		t.Fatal(err)
	}

	if has == true {
		t.Fatal("Key 'foo' should be present")
	}

	tx.Commit()
}
