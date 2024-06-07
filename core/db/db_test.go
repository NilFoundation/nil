package db

import (
	"context"
	"testing"

	common "github.com/NilFoundation/nil/common"
	types "github.com/NilFoundation/nil/core/types"
	"github.com/stretchr/testify/suite"
)

type SuiteBadgerDb struct {
	suite.Suite
	db DB
}

func (suite *SuiteBadgerDb) SetupTest() {
	var err error
	suite.db, err = NewBadgerDb(suite.Suite.T().TempDir())
	suite.Require().NoError(err)
}

func (suite *SuiteBadgerDb) TearDownTest() {
	suite.db.Close()
}

func ValidateTables(t *suite.Suite, db DB) {
	defer t.Require().NoError(db.Put("tbl-1", []byte("foo"), []byte("bar")))

	has, err := db.Exists("tbl-1", []byte("foo"))
	t.Require().NoError(err)
	t.True(has, "Key 'foo' should be present in tbl-1")

	has, err = db.Exists("tbl-2", []byte("foo"))
	t.Require().NoError(err)
	t.False(has, "Key 'foo' should be present in tbl-2")
}

func ValidateTablesName(t *suite.Suite, db DB) {
	t.Require().NoError(db.Put("tbl", []byte("HelloWorld"), []byte("bar1")))
	t.Require().NoError(db.Put("tblHello", []byte("World"), []byte("bar2")))

	val1, err := db.Get("tbl", []byte("HelloWorld"))
	t.Require().NoError(err)
	t.Equal(*val1, []byte("bar1"))

	val2, err := db.Get("tblHello", []byte("World"))
	t.Require().NoError(err)
	t.Equal(*val2, []byte("bar2"))
}

func ValidateTransaction(t *suite.Suite, db DB) {
	ctx := context.Background()
	tx, err := db.CreateRwTx(ctx)
	t.Require().NoError(err)
	defer tx.Rollback()

	tx2, err := db.CreateRwTx(ctx)
	t.Require().NoError(err)
	defer tx2.Rollback()

	t.Require().NoError(tx.Put("tbl", []byte("foo"), []byte("bar")))

	val, err := tx.Get("tbl", []byte("foo"))
	t.Require().NoError(err)
	t.Equal(*val, []byte("bar"))

	_, err = tx.Get("tbl", []byte("bar"))
	t.Require().ErrorIs(err, ErrKeyNotFound)

	has, err := tx.Exists("tbl", []byte("foo"))
	t.Require().NoError(err)
	t.True(has, "Key 'foo' should be present")
	// Testing that parallel transactions don't see changes made by the first one

	has, err = tx2.Exists("tbl", []byte("foo"))
	t.Require().NoError(err)
	t.False(has, "Key 'foo' should not be present")

	tx2.Rollback()
	t.Require().NoError(err)

	err = tx.Commit()
	t.Require().NoError(err)

	// Testing deletion of rows

	tx, err = db.CreateRwTx(ctx)
	t.Require().NoError(err)
	defer tx.Rollback()

	has, err = tx.Exists("tbl", []byte("foo"))

	t.Require().NoError(err)
	t.True(has, "Key 'foo' should be present")

	err = tx.Delete("tbl", []byte("foo"))
	t.Require().NoError(err)

	has, err = tx.Exists("tbl", []byte("foo"))
	t.Require().NoError(err)
	t.False(has, "Key 'foo' should not be present")

	err = tx.Commit()
	t.Require().NoError(err)
}

func ValidateBlock(t *suite.Suite, d DB) {
	ctx := context.Background()

	tx, err := d.CreateRwTx(ctx)
	t.Require().NoError(err)

	block := types.Block{
		Id:                 1,
		PrevBlock:          common.Hash{0x01},
		SmartContractsRoot: common.Hash{0x02},
	}

	err = WriteBlock(tx, types.BaseShardId, &block)
	t.Require().NoError(err)

	block2 := ReadBlock(tx, types.BaseShardId, block.Hash())

	t.Equal(block2.Id, block.Id)
	t.Equal(block2.PrevBlock, block.PrevBlock)
	t.Equal(block2.SmartContractsRoot, block.SmartContractsRoot)
}

func ValidateDbOperations(t *suite.Suite, d DB) {
	t.Require().NoError(d.Put("tbl", []byte("foo"), []byte("bar")))

	val, err := d.Get("tbl", []byte("foo"))
	t.Require().NoError(err)
	t.Equal(*val, []byte("bar"))

	_, err = d.Get("tbl", []byte("bar"))
	t.Require().ErrorIs(err, ErrKeyNotFound)

	has, err := d.Exists("tbl", []byte("foo"))
	t.Require().NoError(err)
	t.True(has, "Key 'foo' should be present")

	t.Require().NoError(d.Delete("tbl", []byte("foo")))

	has, err = d.Exists("tbl", []byte("foo"))
	t.Require().NoError(err)
	t.False(has, "Key 'foo' should not be present")
}

func (suite *SuiteBadgerDb) TestTwoParallelTransaction() {
	ctx := context.Background()
	tx, err := suite.db.CreateRwTx(ctx)
	suite.Suite.Require().NoError(err)
	defer tx.Rollback()

	suite.Suite.Require().NoError(tx.Put("tbl", []byte("foo1"), []byte("bar1")))
	suite.Suite.Require().NoError(tx.Put("tbl", []byte("foo2"), []byte("bar2")))
	suite.Suite.Require().NoError(tx.Commit())

	tx1, err := suite.db.CreateRoTx(ctx)
	suite.Suite.Require().NoError(err)
	defer tx1.Rollback()

	tx2, err := suite.db.CreateRwTx(ctx)
	suite.Suite.Require().NoError(err)
	defer tx2.Rollback()

	_, err = tx1.Get("tbl", []byte("foo2"))
	suite.Suite.Require().NoError(err)

	suite.Suite.Require().NoError(tx2.Put("tbl", []byte("foo2"), []byte("bar22")))
	suite.Suite.Require().NoError(tx2.Commit())
	suite.Suite.Require().NoError(tx1.Commit())
}

func (suite *SuiteBadgerDb) TestValidateTables() {
	ValidateTables(&suite.Suite, suite.db)
}

func (suite *SuiteBadgerDb) TestValidateTablesName() {
	ValidateTablesName(&suite.Suite, suite.db)
}

func (suite *SuiteBadgerDb) TestValidateTransaction() {
	ValidateTransaction(&suite.Suite, suite.db)
}

func (suite *SuiteBadgerDb) TesValidateBlock() {
	ValidateBlock(&suite.Suite, suite.db)
}

func (suite *SuiteBadgerDb) TestValidateDbOperations() {
	ValidateDbOperations(&suite.Suite, suite.db)
}

func (suite *SuiteBadgerDb) fillData(tbl string) {
	suite.T().Helper()

	t := TableName(tbl)
	tg := TableName(tbl + "garbage")
	// Insert some dupsorted records
	suite.Require().NoError(suite.db.Put(t, []byte("key0"), []byte("value0.1")))
	suite.Require().NoError(suite.db.Put(t, []byte("key1"), []byte("value1.1")))
	suite.Require().NoError(suite.db.Put(t, []byte("key3"), []byte("value3.1")))
	suite.Require().NoError(suite.db.Put(t, []byte("key4"), []byte("value4.1")))
	suite.Require().NoError(suite.db.Put(tg, []byte("key0"), []byte("value0.3")))
	suite.Require().NoError(suite.db.Put(tg, []byte("key2"), []byte("value1.3")))
	suite.Require().NoError(suite.db.Put(tg, []byte("key3"), []byte("value2.3")))
	suite.Require().NoError(suite.db.Put(tg, []byte("key4"), []byte("value4.3")))
}

func (suite *SuiteBadgerDb) TestRange() {
	db := suite.db
	ctx := context.Background()

	suite.Run("simple", func() {
		tx, _ := db.CreateRwTx(ctx)
		defer tx.Rollback()
		_ = tx.Put("first", []byte{1}, []byte{1})
		_ = tx.Put("first", []byte{3}, []byte{1})
		_ = tx.Put("first", []byte{4}, []byte{1})
		_ = tx.Put("second", []byte{2}, []byte{8})
		_ = tx.Put("second", []byte{3}, []byte{9})

		keys1 := make([][]byte, 0, 3)
		value1 := make([][]byte, 0, 3)
		keys2 := make([][]byte, 0, 3)
		value2 := make([][]byte, 0, 3)

		it, _ := tx.Range("first", nil, nil)
		for it.HasNext() {
			k, v, err := it.Next()
			suite.Require().NoError(err)
			keys1 = append(keys1, k)
			value1 = append(value1, v)
		}
		it.Close()

		it2, _ := tx.Range("second", nil, nil)
		for it2.HasNext() {
			k, v, err := it2.Next()
			suite.Require().NoError(err)
			keys2 = append(keys2, k)
			value2 = append(value2, v)
		}
		it2.Close()

		suite.Equal([][]byte{{1}, {3}, {4}}, keys1)
		suite.Equal([][]byte{{1}, {1}, {1}}, value1)
		suite.Equal([][]byte{{2}, {3}}, keys2)
		suite.Equal([][]byte{{8}, {9}}, value2)

		suite.Require().NoError(tx.Commit())
	})
	suite.Run("empty", func() {
		tx, _ := db.CreateRwTx(ctx)
		defer tx.Rollback()

		keys := make([][]byte, 0, 3)
		value := make([][]byte, 0, 3)

		it, err := tx.Range("empty", nil, nil)
		suite.Require().NoError(err)
		for it.HasNext() {
			k, v, err := it.Next()
			suite.Require().NoError(err)
			keys = append(keys, k)
			value = append(value, v)
		}
		it.Close()

		suite.Equal([][]byte{}, keys)
		suite.Equal([][]byte{}, value)

		suite.Require().NoError(tx.Commit())
	})
	suite.Run("from-to", func() {
		suite.fillData("from-to")

		tx, err := db.CreateRoTx(ctx)
		suite.Require().NoError(err)
		defer tx.Rollback()

		it, err := tx.Range("from-to", []byte("key1"), []byte("key3"))
		suite.Require().NoError(err)

		suite.True(it.HasNext())
		k, v, err := it.Next()
		suite.Require().NoError(err)
		suite.Equal("key1", string(k))
		suite.Equal("value1.1", string(v))

		suite.True(it.HasNext())
		k, v, err = it.Next()
		suite.Require().NoError(err)
		suite.Equal("key3", string(k))
		suite.Equal("value3.1", string(v))

		suite.False(it.HasNext())
		suite.False(it.HasNext())

		it.Close()
		suite.Require().NoError(tx.Commit())
	})
	suite.Run("from-inf", func() {
		suite.fillData("from-inf")

		tx, err := db.CreateRoTx(ctx)
		suite.Require().NoError(err)
		defer tx.Rollback()

		it, err := tx.Range("from-inf", []byte("key1"), nil)
		suite.Require().NoError(err)

		suite.True(it.HasNext())
		k, v, err := it.Next()
		suite.Require().NoError(err)
		suite.Equal("key1", string(k))
		suite.Equal("value1.1", string(v))

		suite.True(it.HasNext())
		k, v, err = it.Next()
		suite.Require().NoError(err)
		suite.Equal("key3", string(k))
		suite.Equal("value3.1", string(v))

		suite.True(it.HasNext())
		k, v, err = it.Next()
		suite.Require().NoError(err)
		suite.Equal("key4", string(k))
		suite.Equal("value4.1", string(v))

		suite.False(it.HasNext())
		suite.False(it.HasNext())

		it.Close()
		suite.Require().NoError(tx.Commit())
	})
	suite.Run("inf-to", func() {
		suite.fillData("inf-to")

		tx, err := db.CreateRoTx(ctx)
		suite.Require().NoError(err)
		defer tx.Rollback()

		it, err := tx.Range("inf-to", nil, []byte("key1"))
		suite.Require().NoError(err)

		suite.True(it.HasNext())
		k, v, err := it.Next()
		suite.Require().NoError(err)
		suite.Equal("key0", string(k))
		suite.Equal("value0.1", string(v))

		suite.True(it.HasNext())
		k, v, err = it.Next()
		suite.Require().NoError(err)
		suite.Equal("key1", string(k))
		suite.Equal("value1.1", string(v))

		suite.False(it.HasNext())
		suite.False(it.HasNext())

		it.Close()
		suite.Require().NoError(tx.Commit())
	})
}

func TestSuiteBadgerDb(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteBadgerDb))
}
