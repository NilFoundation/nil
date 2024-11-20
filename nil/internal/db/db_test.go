package db

import (
	"bytes"
	"context"
	"sync"
	"testing"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/stretchr/testify/suite"
)

type SuiteBadgerDb struct {
	suite.Suite

	ctx context.Context
	db  DB
}

func (s *SuiteBadgerDb) SetupSuite() {
	s.ctx = context.Background()
}

func (s *SuiteBadgerDb) SetupTest() {
	var err error
	s.db, err = NewBadgerDb(s.Suite.T().TempDir())
	s.Require().NoError(err)
}

func (s *SuiteBadgerDb) TearDownTest() {
	s.db.Close()
}

func ValidateTables(s *suite.Suite, db DB) {
	ctx := context.Background()

	s.Run("put", func() {
		tx, err := db.CreateRwTx(ctx)
		s.Require().NoError(err)
		defer tx.Rollback()

		s.Require().NoError(tx.Put("tbl-1", []byte("foo"), []byte("bar")))
		s.Require().NoError(tx.Commit())
	})

	s.Run("exist", func() {
		tx, err := db.CreateRoTx(ctx)
		s.Require().NoError(err)
		defer tx.Rollback()

		has, err := tx.Exists("tbl-1", []byte("foo"))
		s.Require().NoError(err)
		s.True(has, "Key 'foo' should be present in tbl-1")

		has, err = tx.Exists("tbl-2", []byte("foo"))
		s.Require().NoError(err)
		s.False(has, "Key 'foo' should be present in tbl-2")
	})
}

func ValidateTablesName(s *suite.Suite, db DB) {
	ctx := context.Background()

	s.Run("put", func() {
		tx, err := db.CreateRwTx(ctx)
		s.Require().NoError(err)
		defer tx.Rollback()

		s.Require().NoError(tx.Put("tbl", []byte("HelloWorld"), []byte("bar1")))
		s.Require().NoError(tx.Put("tblHello", []byte("World"), []byte("bar2")))
		s.Require().NoError(tx.Commit())
	})

	s.Run("get", func() {
		tx, err := db.CreateRoTx(ctx)
		s.Require().NoError(err)
		defer tx.Rollback()

		val1, err := tx.Get("tbl", []byte("HelloWorld"))
		s.Require().NoError(err)
		s.Equal([]byte("bar1"), val1)

		val2, err := tx.Get("tblHello", []byte("World"))
		s.Require().NoError(err)
		s.Equal([]byte("bar2"), val2)
	})
}

func ValidateTransaction(s *suite.Suite, db DB) {
	ctx := context.Background()
	tx, err := db.CreateRwTx(ctx)
	s.Require().NoError(err)
	defer tx.Rollback()

	tx2, err := db.CreateRwTx(ctx)
	s.Require().NoError(err)
	defer tx2.Rollback()

	s.Require().NoError(tx.Put("tbl", []byte("foo"), []byte("bar")))

	val, err := tx.Get("tbl", []byte("foo"))
	s.Require().NoError(err)
	s.Equal([]byte("bar"), val)

	_, err = tx.Get("tbl", []byte("bar"))
	s.Require().ErrorIs(err, ErrKeyNotFound)

	has, err := tx.Exists("tbl", []byte("foo"))
	s.Require().NoError(err)
	s.True(has, "Key 'foo' should be present")
	// Testing that parallel transactions don't see changes made by the first one

	has, err = tx2.Exists("tbl", []byte("foo"))
	s.Require().NoError(err)
	s.False(has, "Key 'foo' should not be present")

	tx2.Rollback()
	s.Require().NoError(err)

	err = tx.Commit()
	s.Require().NoError(err)

	// Testing deletion of rows

	tx, err = db.CreateRwTx(ctx)
	s.Require().NoError(err)
	defer tx.Rollback()

	has, err = tx.Exists("tbl", []byte("foo"))
	s.Require().NoError(err)
	s.True(has, "Key 'foo' should be present")

	err = tx.Delete("tbl", []byte("foo"))
	s.Require().NoError(err)

	has, err = tx.Exists("tbl", []byte("foo"))
	s.Require().NoError(err)
	s.False(has, "Key 'foo' should not be present")

	err = tx.Commit()
	s.Require().NoError(err)
}

func ValidateBlock(s *suite.Suite, d DB) {
	ctx := context.Background()

	tx, err := d.CreateRwTx(ctx)
	s.Require().NoError(err)
	defer tx.Rollback()

	block := types.Block{
		Id:                 1,
		PrevBlock:          common.Hash{0x01},
		SmartContractsRoot: common.Hash{0x02},
	}
	blockHash := block.Hash(types.BaseShardId)
	err = WriteBlock(tx, types.BaseShardId, blockHash, &block)
	s.Require().NoError(err)

	block2, err := ReadBlock(tx, types.BaseShardId, blockHash)
	s.Require().NoError(err)
	s.Equal(block2.Id, block.Id)
	s.Equal(block2.PrevBlock, block.PrevBlock)
	s.Equal(block2.SmartContractsRoot, block.SmartContractsRoot)
}

func ValidateDbOperations(s *suite.Suite, d DB) {
	ctx := context.Background()

	s.Run("put", func() {
		tx, err := d.CreateRwTx(ctx)
		s.Require().NoError(err)
		defer tx.Rollback()

		s.Require().NoError(tx.Put("tbl", []byte("foo"), []byte("bar")))
		s.Require().NoError(tx.Commit())
	})

	s.Run("get", func() {
		roTx, err := d.CreateRoTx(ctx)
		s.Require().NoError(err)
		defer roTx.Rollback()

		val, err := roTx.Get("tbl", []byte("foo"))
		s.Require().NoError(err)
		s.Equal([]byte("bar"), val)

		_, err = roTx.Get("tbl", []byte("bar"))
		s.Require().ErrorIs(err, ErrKeyNotFound)

		has, err := roTx.Exists("tbl", []byte("foo"))
		s.Require().NoError(err)
		s.True(has, "Key 'foo' should be present")
	})

	s.Run("delete", func() {
		tx, err := d.CreateRwTx(ctx)
		s.Require().NoError(err)
		defer tx.Rollback()

		s.Require().NoError(tx.Delete("tbl", []byte("foo")))
		s.Require().NoError(tx.Commit())
	})

	s.Run("non-existent", func() {
		roTx, err := d.CreateRoTx(ctx)
		s.Require().NoError(err)
		defer roTx.Rollback()

		has, err := roTx.Exists("tbl", []byte("foo"))
		s.Require().NoError(err)
		s.False(has, "Key 'foo' should not be present")
	})
}

func (s *SuiteBadgerDb) TestTwoParallelTransaction() {
	ctx := context.Background()

	tx, err := s.db.CreateRwTx(ctx)
	s.Suite.Require().NoError(err)
	defer tx.Rollback()

	s.Suite.Require().NoError(tx.Put("tbl", []byte("foo1"), []byte("bar1")))
	s.Suite.Require().NoError(tx.Put("tbl", []byte("foo2"), []byte("bar2")))
	s.Suite.Require().NoError(tx.Commit())

	tx1, err := s.db.CreateRoTx(ctx)
	s.Suite.Require().NoError(err)
	defer tx1.Rollback()

	tx2, err := s.db.CreateRwTx(ctx)
	s.Suite.Require().NoError(err)
	defer tx2.Rollback()

	_, err = tx1.Get("tbl", []byte("foo2"))
	s.Suite.Require().NoError(err)

	s.Suite.Require().NoError(tx2.Put("tbl", []byte("foo2"), []byte("bar22")))
	s.Suite.Require().NoError(tx2.Commit())
}

func (s *SuiteBadgerDb) TestValidateTables() {
	ValidateTables(&s.Suite, s.db)
}

func (s *SuiteBadgerDb) TestValidateTablesName() {
	ValidateTablesName(&s.Suite, s.db)
}

func (s *SuiteBadgerDb) TestValidateTransaction() {
	ValidateTransaction(&s.Suite, s.db)
}

func (s *SuiteBadgerDb) TestValidateBlock() {
	ValidateBlock(&s.Suite, s.db)
}

func (s *SuiteBadgerDb) TestValidateDbOperations() {
	ValidateDbOperations(&s.Suite, s.db)
}

func (s *SuiteBadgerDb) fillData(tbl string) {
	s.T().Helper()

	tx, err := s.db.CreateRwTx(s.ctx)
	s.Require().NoError(err)
	defer tx.Rollback()

	t := TableName(tbl)
	tg := TableName(tbl + "garbage")
	// Insert some dupsorted records
	s.Require().NoError(tx.Put(t, []byte("key0"), []byte("value0.1")))
	s.Require().NoError(tx.Put(t, []byte("key1"), []byte("value1.1")))
	s.Require().NoError(tx.Put(t, []byte("key3"), []byte("value3.1")))
	s.Require().NoError(tx.Put(t, []byte("key4"), []byte("value4.1")))
	s.Require().NoError(tx.Put(tg, []byte("key0"), []byte("value0.3")))
	s.Require().NoError(tx.Put(tg, []byte("key2"), []byte("value1.3")))
	s.Require().NoError(tx.Put(tg, []byte("key3"), []byte("value2.3")))
	s.Require().NoError(tx.Put(tg, []byte("key4"), []byte("value4.3")))

	s.Require().NoError(tx.Commit())
}

func (s *SuiteBadgerDb) TestRange() {
	db := s.db
	ctx := context.Background()

	s.Run("simple", func() {
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
			s.Require().NoError(err)
			keys1 = append(keys1, k)
			value1 = append(value1, v)
		}
		it.Close()

		it2, _ := tx.Range("second", nil, nil)
		for it2.HasNext() {
			k, v, err := it2.Next()
			s.Require().NoError(err)
			keys2 = append(keys2, k)
			value2 = append(value2, v)
		}
		it2.Close()

		s.Equal([][]byte{{1}, {3}, {4}}, keys1)
		s.Equal([][]byte{{1}, {1}, {1}}, value1)
		s.Equal([][]byte{{2}, {3}}, keys2)
		s.Equal([][]byte{{8}, {9}}, value2)

		s.Require().NoError(tx.Commit())
	})
	s.Run("empty", func() {
		tx, _ := db.CreateRwTx(ctx)
		defer tx.Rollback()

		keys := make([][]byte, 0, 3)
		value := make([][]byte, 0, 3)

		it, err := tx.Range("empty", nil, nil)
		s.Require().NoError(err)
		for it.HasNext() {
			k, v, err := it.Next()
			s.Require().NoError(err)
			keys = append(keys, k)
			value = append(value, v)
		}
		it.Close()

		s.Equal([][]byte{}, keys)
		s.Equal([][]byte{}, value)

		s.Require().NoError(tx.Commit())
	})
	s.Run("from-to", func() {
		s.fillData("from-to")

		tx, err := db.CreateRoTx(ctx)
		s.Require().NoError(err)
		defer tx.Rollback()

		it, err := tx.Range("from-to", []byte("key1"), []byte("key3"))
		s.Require().NoError(err)

		s.True(it.HasNext())
		k, v, err := it.Next()
		s.Require().NoError(err)
		s.Equal("key1", string(k))
		s.Equal("value1.1", string(v))

		s.True(it.HasNext())
		k, v, err = it.Next()
		s.Require().NoError(err)
		s.Equal("key3", string(k))
		s.Equal("value3.1", string(v))

		s.False(it.HasNext())
		s.False(it.HasNext())

		it.Close()
	})
	s.Run("from-inf", func() {
		s.fillData("from-inf")

		tx, err := db.CreateRoTx(ctx)
		s.Require().NoError(err)
		defer tx.Rollback()

		it, err := tx.Range("from-inf", []byte("key1"), nil)
		s.Require().NoError(err)

		s.True(it.HasNext())
		k, v, err := it.Next()
		s.Require().NoError(err)
		s.Equal("key1", string(k))
		s.Equal("value1.1", string(v))

		s.True(it.HasNext())
		k, v, err = it.Next()
		s.Require().NoError(err)
		s.Equal("key3", string(k))
		s.Equal("value3.1", string(v))

		s.True(it.HasNext())
		k, v, err = it.Next()
		s.Require().NoError(err)
		s.Equal("key4", string(k))
		s.Equal("value4.1", string(v))

		s.False(it.HasNext())
		s.False(it.HasNext())

		it.Close()
	})
	s.Run("inf-to", func() {
		s.fillData("inf-to")

		tx, err := db.CreateRoTx(ctx)
		s.Require().NoError(err)
		defer tx.Rollback()

		it, err := tx.Range("inf-to", nil, []byte("key1"))
		s.Require().NoError(err)

		s.True(it.HasNext())
		k, v, err := it.Next()
		s.Require().NoError(err)
		s.Equal("key0", string(k))
		s.Equal("value0.1", string(v))

		s.True(it.HasNext())
		k, v, err = it.Next()
		s.Require().NoError(err)
		s.Equal("key1", string(k))
		s.Equal("value1.1", string(v))

		s.False(it.HasNext())
		s.False(it.HasNext())

		it.Close()
	})
}

func (s *SuiteBadgerDb) TestTimestamps() {
	db := s.db
	ctx := context.Background()
	var ts Timestamp

	s.Run("put", func() {
		tx, err := db.CreateRwTx(ctx)
		s.Require().NoError(err)
		defer tx.Rollback()

		s.Require().Equal(Timestamp(0), tx.ReadTimestamp())

		s.Require().NoError(tx.Put("tbl", []byte("foo"), []byte("bar")))
		ts, err = tx.CommitWithTs()
		s.Require().NoError(err)
	})

	s.Run("read-timestamp", func() {
		tx, err := db.CreateRoTxAt(ctx, Timestamp(0))
		s.Require().NoError(err)
		defer tx.Rollback()

		s.Require().Equal(Timestamp(0), tx.ReadTimestamp())

		exists, err := tx.Exists("tbl", []byte("foo"))
		s.Require().NoError(err)
		s.Require().False(exists)
	})

	s.Run("put2", func() {
		tx, err := db.CreateRwTx(ctx)
		s.Require().NoError(err)
		defer tx.Rollback()

		s.Require().Equal(ts, tx.ReadTimestamp())

		val, err := tx.Get("tbl", []byte("foo"))
		s.Require().NoError(err)
		s.Require().Equal([]byte("bar"), val)

		s.Require().NoError(tx.Put("tbl", []byte("foo"), []byte("bar2")))
		s.Require().NoError(tx.Commit())
	})
}

func (s *SuiteBadgerDb) TestStreamLoad() {
	s.fillData("t")

	s.Run("dump and load all data", func() {
		var buf bytes.Buffer
		err := s.db.Stream(s.ctx, func([]byte) bool { return true }, &buf)
		s.Require().NoError(err)

		newDb, err := NewBadgerDbInMemory()
		s.Require().NoError(err)
		defer newDb.Close()

		err = newDb.Fetch(s.ctx, &buf)
		s.Require().NoError(err)

		tx, err := newDb.CreateRoTx(s.ctx)
		s.Require().NoError(err)
		defer tx.Rollback()

		v, err := tx.Get("t", []byte("key0"))
		s.Require().NoError(err)
		s.EqualValues("value0.1", v)

		v, err = tx.Get("tgarbage", []byte("key0"))
		s.Require().NoError(err)
		s.EqualValues("value0.3", v)
	})

	s.Run("dump and load with filter", func() {
		var buf bytes.Buffer
		err := s.db.Stream(s.ctx, func(b []byte) bool {
			return bytes.HasPrefix(b, []byte("t:"))
		}, &buf)
		s.Require().NoError(err)

		newDb, err := NewBadgerDbInMemory()
		s.Require().NoError(err)
		defer newDb.Close()

		err = newDb.Fetch(s.ctx, &buf)
		s.Require().NoError(err)

		tx, err := newDb.CreateRoTx(s.ctx)
		s.Require().NoError(err)
		defer tx.Rollback()

		_, err = tx.Get("tgarbage", []byte("key0"))
		s.Require().ErrorIs(err, ErrKeyNotFound)

		v, err := tx.Get("t", []byte("key0"))
		s.Require().NoError(err)
		s.EqualValues("value0.1", v)

		v, err = tx.Get("t", []byte("key1"))
		s.Require().NoError(err)
		s.EqualValues("value1.1", v)
	})

	s.Run("dump and load all data in parallel", func() {
		var buf1, buf2 bytes.Buffer
		var err error

		err = s.db.Stream(s.ctx, func(key []byte) bool {
			return bytes.HasPrefix(key, []byte("tgarbage:"))
		}, &buf1)
		s.Require().NoError(err)

		err = s.db.Stream(s.ctx, func(key []byte) bool {
			return bytes.HasPrefix(key, []byte("t:"))
		}, &buf2)
		s.Require().NoError(err)

		newDb, err := NewBadgerDbInMemory()
		s.Require().NoError(err)
		defer newDb.Close()

		var wg sync.WaitGroup
		wg.Add(2)

		go func() {
			err := newDb.Fetch(s.ctx, &buf1)
			s.NoError(err)
			wg.Done()
		}()

		go func() {
			err := newDb.Fetch(s.ctx, &buf2)
			s.NoError(err)
			wg.Done()
		}()

		wg.Wait()
		tx, err := newDb.CreateRoTx(s.ctx)
		s.Require().NoError(err)
		defer tx.Rollback()

		v, err := tx.Get("t", []byte("key0"))
		s.Require().NoError(err)
		s.EqualValues("value0.1", v)

		v, err = tx.Get("tgarbage", []byte("key0"))
		s.Require().NoError(err)
		s.EqualValues("value0.3", v)
	})
}

func TestSuiteBadgerDb(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteBadgerDb))
}
