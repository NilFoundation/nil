package rpctest

import (
	"testing"

	"github.com/NilFoundation/nil/cmd/nil/nilservice"
	"github.com/NilFoundation/nil/core/collate"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/readthroughdb"
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
	s.readthroughdb = readthroughdb.NewReadThroughDb(s.client, s.db)
}

func (s *SuiteReadThrough) TearDownTest() {
	s.cancel()
}

func (s *SuiteReadThrough) TestBasic() {
	ctx := s.context

	s.Run("put", func() {
		tx, err := s.db.CreateRwTx(ctx)
		s.Require().NoError(err)
		defer tx.Rollback()

		for i := range 10 {
			kv := []byte{byte(i)}
			s.Require().NoError(tx.Put("test", kv, kv))
		}
		s.Require().NoError(tx.Commit())
	})

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
}

func TestSuiteReadThrough(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteReadThrough))
}
