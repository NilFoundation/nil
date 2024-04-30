package tests

import (
	"bytes"
	"context"
	"testing"

	common "github.com/NilFoundation/nil/common"
	db "github.com/NilFoundation/nil/core/db"
	types "github.com/NilFoundation/nil/core/types"
	"github.com/stretchr/testify/require"
)

func ValidateTables[T db.DB](t *testing.T, db T) {
	defer db.Close()

	err := db.Set("tbl-1", []byte("foo"), []byte("bar"))

	if err != nil {
		t.Fatal(err)
	}

	has, err := db.Exists("tbl-1", []byte("foo"))

	if err != nil {
		t.Fatal(err)
	}

	if has == false {
		t.Fatal("Key 'foo' should be present in tbl-1")
	}

	has, err = db.Exists("tbl-2", []byte("foo"))

	if err != nil {
		t.Fatal(err)
	}

	if has == true {
		t.Fatal("Key 'foo' should not be present in tbl-2")
	}
}

func ValidateTransaction[T db.DB](t *testing.T, db T) {
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

	has, err := tx.Exists("tbl", []byte("foo"))

	if err != nil {
		t.Fatal(err)
	}

	if has == false {
		t.Fatal("Key 'foo' should be present")
	}

	// Testing that parallel transactions don't see changes made by the first one

	has, err = tx2.Exists("tbl", []byte("foo"))

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

	has, err = tx.Exists("tbl", []byte("foo"))

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

	has, err = tx.Exists("tbl", []byte("foo"))

	if err != nil {
		t.Fatal(err)
	}

	if has == true {
		t.Fatal("Key 'foo' should be present")
	}

	tx.Commit()
}

func ValidateBlock[T db.DB](t *testing.T, d T) {
	defer d.Close()

	ctx := context.Background()

	tx, err := d.BeginTx(ctx)
	require.NoError(t, err)

	block := types.Block{
		Id:                 1,
		PrevBlock:          common.Hash{0x01},
		SmartContractsRoot: common.Hash{0x02},
	}

	err = db.WriteBlock(tx, &block)
	require.NoError(t, err)

	block2 := db.ReadBlock(tx, block.Hash())

	require.Equal(t, block2.Id, block.Id)
	require.Equal(t, block2.PrevBlock, block.PrevBlock)
	require.Equal(t, block2.SmartContractsRoot, block.SmartContractsRoot)
}

func TestTablesBadger(t *testing.T) {
	ValidateTables(t, db.NewBadgerDb(t.TempDir()))
}

func TestTransactionBadger(t *testing.T) {
	ValidateTransaction(t, db.NewBadgerDb(t.TempDir()))
}

func TestBlockBadger(t *testing.T) {
	ValidateBlock(t, db.NewBadgerDb(t.TempDir()))
}

func TestTablesSqlite(t *testing.T) {
	ValidateTables(t, db.NewSqlite(t.TempDir()+"/foo.db"))
}

func TestTransactionSqlite(t *testing.T) {
	ValidateTransaction(t, db.NewSqlite(t.TempDir()+"/foo.db"))
}

func TestBlockSqlite(t *testing.T) {
	ValidateBlock(t, db.NewSqlite(t.TempDir()+"/foo.db"))
}
