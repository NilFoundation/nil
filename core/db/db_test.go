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

type SuiteSqliteDb struct {
	suite.Suite
	db DB
}

func (suite *SuiteBadgerDb) SetupTest() {
	var err error
	suite.db, err = NewBadgerDb(suite.Suite.T().TempDir())
	suite.Require().NoError(err)
}

func (suite *SuiteSqliteDb) SetupTest() {
	var err error
	suite.db, err = NewSqlite(suite.Suite.T().TempDir() + "foo.bar")
	suite.Require().NoError(err)
}

func ValidateTables(t *suite.Suite, db DB) {
	defer db.Close()

	t.Require().NoError(db.Put("tbl-1", []byte("foo"), []byte("bar")))

	has, err := db.Exists("tbl-1", []byte("foo"))

	t.Require().NoError(err)
	t.True(has, "Key 'foo' should be present in tbl-1")

	has, err = db.Exists("tbl-2", []byte("foo"))

	t.Require().NoError(err)
	t.False(has, "Key 'foo' should be present in tbl-2")
}

func ValidateTransaction(t *suite.Suite, db DB) {
	defer db.Close()

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
	defer d.Close()

	ctx := context.Background()

	tx, err := d.CreateRwTx(ctx)
	t.Require().NoError(err)

	block := types.Block{
		Id:                 1,
		PrevBlock:          common.Hash{0x01},
		SmartContractsRoot: common.Hash{0x02},
	}

	err = WriteBlock(tx, &block)
	t.Require().NoError(err)

	block2 := ReadBlock(tx, block.Hash())

	t.Equal(block2.Id, block.Id)
	t.Equal(block2.PrevBlock, block.PrevBlock)
	t.Equal(block2.SmartContractsRoot, block.SmartContractsRoot)
}

func ValidateDbOperations(t *suite.Suite, d DB) {
	defer d.Close()

	t.Require().NoError(d.Put("tbl", []byte("foo"), []byte("bar")))

	val, err := d.Get("tbl", []byte("foo"))

	t.Require().NoError(err)
	t.Equal(*val, []byte("bar"))

	has, err := d.Exists("tbl", []byte("foo"))

	t.Require().NoError(err)
	t.True(has, "Key 'foo' should be present")

	t.Require().NoError(d.Delete("tbl", []byte("foo")))

	has, err = d.Exists("tbl", []byte("foo"))

	t.Require().NoError(err)
	t.False(has, "Key 'foo' should not be present")
}

func (suite *SuiteBadgerDb) TestTwoParallelTwoTransaction() {
	defer suite.db.Close()

	ctx := context.Background()

	tx, err := suite.db.CreateRwTx(ctx)

	suite.Suite.Require().NoError(err)
	defer tx.Rollback()

	suite.Suite.Require().NoError(tx.Put("tbl", []byte("foo1"), []byte("bar1")))
	suite.Suite.Require().NoError(tx.Put("tbl", []byte("foo2"), []byte("bar2")))

	suite.Suite.Require().NoError(tx.Commit())

	tx1, err := suite.db.CreateRoTx(ctx)

	suite.Suite.Require().NoError(err)

	tx2, err := suite.db.CreateRwTx(ctx)
	suite.Suite.Require().NoError(err)

	_, err = tx1.Get("tbl", []byte("foo2"))
	suite.Suite.Require().NoError(err)

	suite.Suite.Require().NoError(tx2.Put("tbl", []byte("foo2"), []byte("bar22")))
	suite.Suite.Require().NoError(tx2.Commit())
	suite.Suite.Require().NoError(tx1.Commit())
}

func (suite *SuiteBadgerDb) TestValidateTables() {
	ValidateTables(&suite.Suite, suite.db)
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

func TestSuiteBadgerDb(t *testing.T) {
	suite.Run(t, new(SuiteBadgerDb))
}

func (suite *SuiteSqliteDb) TestValidateTables() {
	ValidateTables(&suite.Suite, suite.db)
}
func (suite *SuiteSqliteDb) TestValidateTransaction() {
	ValidateTransaction(&suite.Suite, suite.db)
}
func (suite *SuiteSqliteDb) TestValidateBlock() {
	ValidateBlock(&suite.Suite, suite.db)
}
func (suite *SuiteSqliteDb) TestValidateDbOperations() {
	ValidateDbOperations(&suite.Suite, suite.db)
}

func TestSuiteSqliteDb(t *testing.T) {
	suite.Run(t, new(SuiteSqliteDb))
}
