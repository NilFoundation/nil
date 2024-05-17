package filters

import (
	"context"
	"testing"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/types"
	"github.com/stretchr/testify/suite"
)

type SuiteFilters struct {
	suite.Suite
	ctx    context.Context
	cancel context.CancelFunc
	stor   db.DB
}

func (s *SuiteFilters) SetupTest() {
	s.ctx, s.cancel = context.WithCancel(context.Background())
	var err error
	s.stor, err = db.NewBadgerDbInMemory()
	s.Require().NoError(err)
}

func (s *SuiteFilters) TearDownTest() {
	s.cancel()
}

func (s *SuiteFilters) TestMatcherOneReceipt() {
	filters := NewFiltersManager(s.ctx, s.stor, false)
	s.NotNil(filters)

	block := types.Block{Id: 1}

	var receipts []*types.Receipt

	address1 := common.HexToAddress("0x111111111")
	logs := []*types.Log{
		{
			Address: address1,
			Topics:  []common.Hash{{0x01}, {0x02}},
			Data:    []byte{0xaa, 0xaa},
		},
		{
			Address: address1,
			Topics:  []common.Hash{{0x03}, {0x02}, {0x05}},
			Data:    []byte{0xbb, 0xbb},
		},
		{
			Address: address1,
			Topics:  []common.Hash{},
			Data:    []byte{0xcc, 0xcc},
		},
	}

	receipts = append(receipts, &types.Receipt{ContractAddress: address1, Logs: logs})

	// All logs with Address == address1
	id, f := filters.NewFilter(&FilterQuery{Addresses: []common.Address{address1}})
	s.NotEmpty(id)
	s.NotNil(f)

	s.Require().NoError(filters.process(&block, receipts))
	s.Len(f.output, 3)
	s.Equal(<-f.LogsChannel(), logs[0])
	s.Equal(<-f.LogsChannel(), logs[1])
	s.Equal(<-f.LogsChannel(), logs[2])
	filters.RemoveFilter(id)

	// Only logs with [1, 2] topics
	id, f = filters.NewFilter(&FilterQuery{Addresses: []common.Address{address1}, Topics: [][]common.Hash{{{0x01}}, {{0x02}}}})
	s.NotEmpty(id)
	s.NotNil(f)
	s.Require().NoError(filters.process(&block, receipts))
	s.Len(f.output, 1)
	s.Equal(<-f.LogsChannel(), logs[0])
	filters.RemoveFilter(id)

	// Only logs with [any, 2] topics
	id, f = filters.NewFilter(&FilterQuery{Addresses: []common.Address{address1}, Topics: [][]common.Hash{{}, {{0x02}}}})
	s.NotEmpty(id)
	s.NotNil(f)

	s.Require().NoError(filters.process(&block, receipts))
	s.Len(f.output, 2)
	s.Equal(<-f.LogsChannel(), logs[0])
	s.Equal(<-f.LogsChannel(), logs[1])
	filters.RemoveFilter(id)
}

func (s *SuiteFilters) TestMatcherTwoReceipts() {
	filters := NewFiltersManager(s.ctx, s.stor, false)
	s.NotNil(filters)

	block := types.Block{Id: 1}

	var receipts []*types.Receipt

	address1 := common.HexToAddress("0x1111111111")
	address2 := common.HexToAddress("0x2222222222")

	logs1 := []*types.Log{
		{
			Address: address1,
			Topics:  []common.Hash{{0x01}, {0x02}, {0x03}},
			Data:    []byte{0xaa, 0xaa},
		},
		{
			Address: address1,
			Topics:  []common.Hash{{0x03}},
			Data:    []byte{0xbb, 0xbb},
		},
		{
			Address: address1,
			Topics:  []common.Hash{},
			Data:    []byte{0xcc, 0xcc},
		},
		{
			Address: address1,
			Topics:  []common.Hash{{0x03}, {0x04}, {0x03}},
			Data:    []byte{0xaa, 0xaa},
		},
	}
	receipts = append(receipts, &types.Receipt{ContractAddress: address1, Logs: logs1})

	logs2 := []*types.Log{
		{
			Address: address2,
			Topics:  []common.Hash{{0x01}, {0x02}, {0x03}},
			Data:    []byte{0xaa, 0xaa},
		},
		{
			Address: address2,
			Topics:  []common.Hash{{0x03}, {0x01}, {0x03}},
			Data:    []byte{0xbb, 0xbb},
		},
	}
	receipts = append(receipts, &types.Receipt{ContractAddress: address2, Logs: logs2})

	// All logs
	id, f := filters.NewFilter(&FilterQuery{})
	s.NotEmpty(id)
	s.NotNil(f)

	s.Require().NoError(filters.process(&block, receipts))
	s.Len(f.output, 6)
	s.Equal(<-f.LogsChannel(), logs1[0])
	s.Equal(<-f.LogsChannel(), logs1[1])
	s.Equal(<-f.LogsChannel(), logs1[2])
	s.Equal(<-f.LogsChannel(), logs1[3])
	s.Equal(<-f.LogsChannel(), logs2[0])
	s.Equal(<-f.LogsChannel(), logs2[1])
	filters.RemoveFilter(id)

	// All logs of address1
	id, f = filters.NewFilter(&FilterQuery{Addresses: []common.Address{address1}})
	s.NotEmpty(id)
	s.NotNil(f)

	s.Require().NoError(filters.process(&block, receipts))
	s.Len(f.output, 4)
	s.Equal(<-f.LogsChannel(), logs1[0])
	s.Equal(<-f.LogsChannel(), logs1[1])
	s.Equal(<-f.LogsChannel(), logs1[2])
	s.Equal(<-f.LogsChannel(), logs1[3])
	filters.RemoveFilter(id)

	// All logs of address2
	id, f = filters.NewFilter(&FilterQuery{Addresses: []common.Address{address2}})
	s.NotEmpty(id)
	s.NotNil(f)

	s.Require().NoError(filters.process(&block, receipts))
	s.Len(f.output, 2)
	s.Equal(<-f.LogsChannel(), logs2[0])
	s.Equal(<-f.LogsChannel(), logs2[1])
	filters.RemoveFilter(id)

	// address1: nil, nil, 3
	id, f = filters.NewFilter(&FilterQuery{
		Addresses: []common.Address{address1},
		Topics:    [][]common.Hash{{}, {}, {{0x03}}},
	})
	s.NotEmpty(id)
	s.NotNil(f)

	s.Require().NoError(filters.process(&block, receipts))
	s.Require().Len(f.LogsChannel(), 2)
	s.Equal(<-f.LogsChannel(), logs1[0])
	s.Equal(<-f.LogsChannel(), logs1[3])
	filters.RemoveFilter(id)

	// any address: nil, 2
	id, f = filters.NewFilter(&FilterQuery{Topics: [][]common.Hash{{}, {{2}}}})
	s.NotEmpty(id)
	s.NotNil(f)

	s.Require().NoError(filters.process(&block, receipts))
	s.Require().Len(f.LogsChannel(), 2)
	s.Equal(<-f.LogsChannel(), logs1[0])
	s.Equal(<-f.LogsChannel(), logs2[0])
	filters.RemoveFilter(id)

	// any address: nil, 2
	id, f = filters.NewFilter(&FilterQuery{Topics: [][]common.Hash{{{3}}, {}, {{3}}}})
	s.NotEmpty(id)
	s.NotNil(f)

	s.Require().NoError(filters.process(&block, receipts))
	s.Require().Len(f.LogsChannel(), 2)
	s.Equal(<-f.LogsChannel(), logs1[3])
	s.Equal(<-f.LogsChannel(), logs2[1])
	filters.RemoveFilter(id)

	// address1: 3
	id, f = filters.NewFilter(&FilterQuery{
		Addresses: []common.Address{address1},
		Topics:    [][]common.Hash{{{0x03}}},
	})
	s.NotEmpty(id)
	s.NotNil(f)

	s.Require().NoError(filters.process(&block, receipts))
	s.Require().Len(f.LogsChannel(), 2)
	s.Equal(<-f.LogsChannel(), logs1[1])
	s.Equal(<-f.LogsChannel(), logs1[3])
	filters.RemoveFilter(id)

	// any address: 3
	id, f = filters.NewFilter(&FilterQuery{Topics: [][]common.Hash{{{0x03}}}})
	s.NotEmpty(id)
	s.NotNil(f)

	s.Require().NoError(filters.process(&block, receipts))
	s.Require().Len(f.LogsChannel(), 3)
	s.Equal(<-f.LogsChannel(), logs1[1])
	s.Equal(<-f.LogsChannel(), logs1[3])
	s.Equal(<-f.LogsChannel(), logs2[1])
	filters.RemoveFilter(id)
}

func TestFilters(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteFilters))
}
