package rpctest

import (
	"testing"

	"github.com/NilFoundation/nil/nil/internal/collate"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/readthroughdb"
	"github.com/NilFoundation/nil/nil/services/nilservice"
	"github.com/stretchr/testify/suite"
)

type SuiteReadThrough struct {
	RpcSuite

	readthroughdb db.DB
}

func (s *SuiteReadThrough) SetupTest() {
	s.start(&nilservice.Config{
		NShards:              5,
		HttpPort:             8539,
		Topology:             collate.TrivialShardTopologyId,
		CollatorTickPeriodMs: 100,
		GasBasePrice:         10,
	})
	inDb, err := db.NewBadgerDbInMemory()
	s.Require().NoError(err)
	s.readthroughdb, err = readthroughdb.NewReadThroughDb(s.client, inDb)
	s.Require().NoError(err)
}

func (s *SuiteReadThrough) TearDownTest() {
	s.cancel()
}

func (s *SuiteReadThrough) TestBasic() {
	ctx := s.context
	var ts db.Timestamp

	s.Run("put", func() {
		tx, err := s.db.CreateRwTx(ctx)
		s.Require().NoError(err)
		defer tx.Rollback()

		for i := range 10 {
			kv := []byte{byte(i)}
			s.Require().NoError(tx.Put("test", kv, kv))
		}
		s.Require().NoError(tx.Put("test", []byte{100}, nil))
		ts, err = tx.CommitWithTs()
		s.Require().NoError(err)
	})

	s.Require().NoError(s.client.DbInitTimestamp(ts.Uint64()))

	s.Run("read", func() {
		tx, err := s.readthroughdb.CreateRoTx(ctx)
		s.Require().NoError(err)
		defer tx.Rollback()

		for i := range 10 {
			kv := []byte{byte(i)}
			has, err := tx.Exists("test", kv)
			s.Require().NoError(err)
			s.True(has)

			val, err := tx.Get("test", kv)
			s.Require().NoError(err)
			s.Require().Equal(kv, val)
		}

		for i := 10; i < 15; i++ {
			kv := []byte{byte(i)}
			has, err := tx.Exists("test", kv)
			s.Require().NoError(err)
			s.Require().False(has)

			_, err = tx.Get("test", kv)
			s.Require().ErrorIs(err, db.ErrKeyNotFound)
		}

		k := []byte{100}
		has, err := tx.Exists("test", k)
		s.Require().NoError(err)
		s.Require().True(has)

		val, err := tx.Get("test", k)
		s.Require().NoError(err)
		s.Require().Nil(val)
	})

	s.Run("overwrite", func() {
		tx, err := s.readthroughdb.CreateRwTx(ctx)
		s.Require().NoError(err)
		defer tx.Rollback()

		for i := range 10 {
			kv := []byte{byte(i)}
			new_val := []byte{byte(i + 1)}
			s.Require().NoError(tx.Put("test", kv, new_val))
		}
		s.Require().NoError(tx.Commit())
	})

	s.Run("check", func() {
		tx, err := s.readthroughdb.CreateRoTx(ctx)
		s.Require().NoError(err)
		defer tx.Rollback()

		for i := range 10 {
			kv := []byte{byte(i)}
			val := []byte{byte(i + 1)}

			rval, err := tx.Get("test", kv)
			s.Require().NoError(err)
			s.Require().Equal(val, rval)
		}
	})

	s.Run("delete", func() {
		tx, err := s.readthroughdb.CreateRwTx(ctx)
		s.Require().NoError(err)
		defer tx.Rollback()

		for i := range 5 {
			kv := []byte{byte(i)}
			s.Require().NoError(tx.Delete("test", kv))
		}
		s.Require().NoError(tx.Commit())
	})

	s.Run("check", func() {
		tx, err := s.readthroughdb.CreateRoTx(ctx)
		s.Require().NoError(err)
		defer tx.Rollback()

		for i := range 5 {
			kv := []byte{byte(i)}
			_, err := tx.Get("test", kv)
			s.Require().ErrorIs(err, db.ErrKeyNotFound)
		}
		for i := 5; i < 10; i++ {
			key := []byte{byte(i)}
			val := []byte{byte(i + 1)}

			rval, err := tx.Get("test", key)
			s.Require().NoError(err)
			s.Require().Equal(val, rval)
		}
	})
	s.Run("overwrite_after_delete", func() {
		tx, err := s.readthroughdb.CreateRwTx(ctx)
		s.Require().NoError(err)
		defer tx.Rollback()

		for i := range 5 {
			kv := []byte{byte(i)}
			new_val := []byte{byte(i + 1)}
			s.Require().NoError(tx.Put("test", kv, new_val))
		}
		s.Require().NoError(tx.Commit())
	})
	s.Run("check", func() {
		tx, err := s.readthroughdb.CreateRoTx(ctx)
		s.Require().NoError(err)
		defer tx.Rollback()

		for i := range 10 {
			key := []byte{byte(i)}
			val := []byte{byte(i + 1)}

			rval, err := tx.Get("test", key)
			s.Require().NoError(err)
			s.Require().Equal(val, rval)
		}
	})
}

func TestSuiteReadThrough(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteReadThrough))
}
