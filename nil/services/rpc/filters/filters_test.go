package filters

import (
	"context"
	"testing"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/mpt"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/suite"
)

type SuiteFilters struct {
	suite.Suite
	ctx     context.Context
	cancel  context.CancelFunc
	db      db.DB
	filters *FiltersManager
}

func (s *SuiteFilters) SetupTest() {
	s.ctx, s.cancel = context.WithCancel(context.Background())
	var err error
	s.db, err = db.NewBadgerDbInMemory()
	s.Require().NoError(err)
}

func (s *SuiteFilters) TearDownTest() {
	s.cancel()
	s.filters.WaitForShutdown()
	s.db.Close()
}

func (s *SuiteFilters) TestMatcherOneReceipt() {
	filters := NewFiltersManager(s.ctx, s.db, false)
	s.NotNil(filters)
	s.filters = filters

	block := types.Block{BlockData: types.BlockData{Id: 1}}

	var receipts []*types.Receipt

	address1 := types.HexToAddress("0x111111111")
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
	id, f := filters.NewFilter(&FilterQuery{Addresses: []types.Address{address1}})
	s.NotEmpty(id)
	s.NotNil(f)

	s.Require().NoError(filters.process(&block, receipts))
	s.Len(f.output, 3)
	s.Equal((<-f.LogsChannel()).Log, logs[0])
	s.Equal((<-f.LogsChannel()).Log, logs[1])
	s.Equal((<-f.LogsChannel()).Log, logs[2])
	filters.RemoveFilter(id)

	// Only logs with [1, 2] topics
	id, f = filters.NewFilter(
		&FilterQuery{Addresses: []types.Address{address1}, Topics: [][]common.Hash{{{0x01}}, {{0x02}}}})
	s.NotEmpty(id)
	s.NotNil(f)
	s.Require().NoError(filters.process(&block, receipts))
	s.Len(f.output, 1)
	s.Equal((<-f.LogsChannel()).Log, logs[0])
	filters.RemoveFilter(id)

	// Only logs with [any, 2] topics
	id, f = filters.NewFilter(&FilterQuery{Addresses: []types.Address{address1}, Topics: [][]common.Hash{{}, {{0x02}}}})
	s.NotEmpty(id)
	s.NotNil(f)

	s.Require().NoError(filters.process(&block, receipts))
	s.Len(f.output, 2)
	s.Equal((<-f.LogsChannel()).Log, logs[0])
	s.Equal((<-f.LogsChannel()).Log, logs[1])
	filters.RemoveFilter(id)
}

func (s *SuiteFilters) TestMatcherTwoReceipts() {
	filters := NewFiltersManager(s.ctx, s.db, false)
	s.NotNil(filters)
	s.filters = filters

	block := types.Block{BlockData: types.BlockData{Id: 1}}

	var receipts []*types.Receipt

	address1 := types.HexToAddress("0x1111111111")
	address2 := types.HexToAddress("0x2222222222")

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
	s.Equal((<-f.LogsChannel()).Log, logs1[0])
	s.Equal((<-f.LogsChannel()).Log, logs1[1])
	s.Equal((<-f.LogsChannel()).Log, logs1[2])
	s.Equal((<-f.LogsChannel()).Log, logs1[3])
	s.Equal((<-f.LogsChannel()).Log, logs2[0])
	s.Equal((<-f.LogsChannel()).Log, logs2[1])
	filters.RemoveFilter(id)

	// All logs of address1
	id, f = filters.NewFilter(&FilterQuery{Addresses: []types.Address{address1}})
	s.NotEmpty(id)
	s.NotNil(f)

	s.Require().NoError(filters.process(&block, receipts))
	s.Len(f.output, 4)
	s.Equal((<-f.LogsChannel()).Log, logs1[0])
	s.Equal((<-f.LogsChannel()).Log, logs1[1])
	s.Equal((<-f.LogsChannel()).Log, logs1[2])
	s.Equal((<-f.LogsChannel()).Log, logs1[3])
	filters.RemoveFilter(id)

	// All logs of address2
	id, f = filters.NewFilter(&FilterQuery{Addresses: []types.Address{address2}})
	s.NotEmpty(id)
	s.NotNil(f)

	s.Require().NoError(filters.process(&block, receipts))
	s.Len(f.output, 2)
	s.Equal((<-f.LogsChannel()).Log, logs2[0])
	s.Equal((<-f.LogsChannel()).Log, logs2[1])
	filters.RemoveFilter(id)

	// address1: nil, nil, 3
	id, f = filters.NewFilter(&FilterQuery{
		Addresses: []types.Address{address1},
		Topics:    [][]common.Hash{{}, {}, {{0x03}}},
	})
	s.NotEmpty(id)
	s.NotNil(f)

	s.Require().NoError(filters.process(&block, receipts))
	s.Require().Len(f.LogsChannel(), 2)
	s.Equal((<-f.LogsChannel()).Log, logs1[0])
	s.Equal((<-f.LogsChannel()).Log, logs1[3])
	filters.RemoveFilter(id)

	// any address: nil, 2
	id, f = filters.NewFilter(&FilterQuery{Topics: [][]common.Hash{{}, {{2}}}})
	s.NotEmpty(id)
	s.NotNil(f)

	s.Require().NoError(filters.process(&block, receipts))
	s.Require().Len(f.LogsChannel(), 2)
	s.Equal((<-f.LogsChannel()).Log, logs1[0])
	s.Equal((<-f.LogsChannel()).Log, logs2[0])
	filters.RemoveFilter(id)

	// any address: nil, 2
	id, f = filters.NewFilter(&FilterQuery{Topics: [][]common.Hash{{{3}}, {}, {{3}}}})
	s.NotEmpty(id)
	s.NotNil(f)

	s.Require().NoError(filters.process(&block, receipts))
	s.Require().Len(f.LogsChannel(), 2)
	s.Equal((<-f.LogsChannel()).Log, logs1[3])
	s.Equal((<-f.LogsChannel()).Log, logs2[1])
	filters.RemoveFilter(id)

	// address1: 3
	id, f = filters.NewFilter(&FilterQuery{
		Addresses: []types.Address{address1},
		Topics:    [][]common.Hash{{{0x03}}},
	})
	s.NotEmpty(id)
	s.NotNil(f)

	s.Require().NoError(filters.process(&block, receipts))
	s.Require().Len(f.LogsChannel(), 2)
	s.Equal((<-f.LogsChannel()).Log, logs1[1])
	s.Equal((<-f.LogsChannel()).Log, logs1[3])
	filters.RemoveFilter(id)

	// any address: 3
	id, f = filters.NewFilter(&FilterQuery{Topics: [][]common.Hash{{{0x03}}}})
	s.NotEmpty(id)
	s.NotNil(f)

	s.Require().NoError(filters.process(&block, receipts))
	s.Require().Len(f.LogsChannel(), 3)
	s.Equal((<-f.LogsChannel()).Log, logs1[1])
	s.Equal((<-f.LogsChannel()).Log, logs1[3])
	s.Equal((<-f.LogsChannel()).Log, logs2[1])
	filters.RemoveFilter(id)
}

func (s *SuiteFilters) TestBlocksRange() {
	tx, err := s.db.CreateRwTx(s.ctx)
	s.Require().NoError(err)
	defer tx.Rollback()

	filters := NewFiltersManager(s.ctx, s.db, true)
	s.NotNil(filters)
	s.filters = filters
	address := types.HexToAddress("0x1111111111")

	receiptsMpt := mpt.NewDbMPT(tx, 0, db.ReceiptTrieTable)

	logsInput := []*types.Log{
		{
			Address: address,
			Topics:  []common.Hash{{0x03}, {0x02}},
			Data:    []byte{1},
		},
		{
			Address: address,
			Topics:  []common.Hash{{0x04}, {0x02}},
			Data:    []byte{2},
		},
	}

	receipt := &types.Receipt{ContractAddress: address, Logs: logsInput}
	receiptEncoded, err := receipt.MarshalSSZ()
	s.Require().NoError(err)
	key, err := receipt.HashTreeRoot()
	s.Require().NoError(err)
	s.Require().NoError(receiptsMpt.Set(key[:], receiptEncoded))

	block := types.Block{
		BlockData: types.BlockData{
			Id:           0,
			ReceiptsRoot: receiptsMpt.RootHash(),
		},
	}
	blockHash := block.Hash(types.MainShardId)
	s.Require().NoError(db.WriteBlock(tx, types.MainShardId, blockHash, &block))
	blockResult := &execution.BlockGenerationResult{BlockHash: blockHash, Block: &block}
	err = execution.PostprocessBlock(tx, types.MainShardId, blockResult, execution.ModeVerify)
	s.Require().NoError(err)

	block = types.Block{
		BlockData: types.BlockData{
			Id:           1,
			ReceiptsRoot: receiptsMpt.RootHash(),
		},
	}
	blockHash = block.Hash(types.MainShardId)
	s.Require().NoError(db.WriteBlock(tx, types.MainShardId, blockHash, &block))
	blockResult = &execution.BlockGenerationResult{BlockHash: blockHash, Block: &block}
	err = execution.PostprocessBlock(tx, types.MainShardId, blockResult, execution.ModeVerify)
	s.Require().NoError(err)

	block = types.Block{
		BlockData: types.BlockData{
			Id:           2,
			ReceiptsRoot: receiptsMpt.RootHash(),
		},
	}
	blockHash = block.Hash(types.MainShardId)
	s.Require().NoError(db.WriteBlock(tx, types.MainShardId, blockHash, &block))
	blockResult = &execution.BlockGenerationResult{BlockHash: blockHash, Block: &block}
	err = execution.PostprocessBlock(tx, types.MainShardId, blockResult, execution.ModeVerify)
	s.Require().NoError(err)

	block = types.Block{
		BlockData: types.BlockData{
			Id:           3,
			ReceiptsRoot: receiptsMpt.RootHash(),
		},
	}
	blockHash = block.Hash(types.MainShardId)
	s.Require().NoError(db.WriteBlock(tx, types.MainShardId, blockHash, &block))
	blockResult = &execution.BlockGenerationResult{BlockHash: blockHash, Block: &block}
	err = execution.PostprocessBlock(tx, types.MainShardId, blockResult, execution.ModeVerify)
	s.Require().NoError(err)
	s.Require().NoError(tx.Commit())

	topics := [][]common.Hash{{{3}}}
	query := &FilterQuery{
		BlockHash: nil,
		FromBlock: uint256.NewInt(1),
		ToBlock:   uint256.NewInt(2),
		Addresses: []types.Address{address},
		Topics:    topics,
	}
	id1, filter1 := filters.NewFilter(query)
	s.Require().NotNil(filter1)
	s.Require().NotEmpty(id1)

	s.Len(filter1.output, 2)
	s.Equal(logsInput[0], (<-filter1.LogsChannel()).Log)
	s.Equal(logsInput[0], (<-filter1.LogsChannel()).Log)

	topics = [][]common.Hash{{{3}}}
	query = &FilterQuery{
		BlockHash: nil,
		FromBlock: uint256.NewInt(1),
		ToBlock:   nil,
		Addresses: []types.Address{address},
		Topics:    topics,
	}
	id2, filter2 := filters.NewFilter(query)
	s.Require().NotNil(filter2)
	s.Require().NotEmpty(id2)

	s.Len(filter2.output, 3)
	s.Equal(logsInput[0], (<-filter2.LogsChannel()).Log)
	s.Equal(logsInput[0], (<-filter2.LogsChannel()).Log)
	s.Equal(logsInput[0], (<-filter2.LogsChannel()).Log)

	// Check with toBlock but without fromBlock
	query = &FilterQuery{
		BlockHash: nil,
		FromBlock: nil,
		ToBlock:   uint256.NewInt(0),
		Addresses: []types.Address{address},
		Topics:    topics,
	}
	id3, filter3 := filters.NewFilter(query)
	s.Require().NotNil(filter3)
	s.Require().NotEmpty(id3)

	s.Len(filter3.output, 1)
	s.Equal(logsInput[0], (<-filter3.LogsChannel()).Log)

	tx, err = s.db.CreateRwTx(s.ctx)
	s.Require().NoError(err)
	defer tx.Rollback()
	block = types.Block{
		BlockData: types.BlockData{
			Id:           4,
			ReceiptsRoot: receiptsMpt.RootHash(),
		},
	}
	blockHash = block.Hash(types.MainShardId)
	s.Require().NoError(db.WriteBlock(tx, types.MainShardId, blockHash, &block))
	blockResult = &execution.BlockGenerationResult{BlockHash: blockHash, Block: &block}
	err = execution.PostprocessBlock(tx, types.MainShardId, blockResult, execution.ModeVerify)
	s.Require().NoError(err)
	s.Require().NoError(tx.Commit())

	// Check that only filter2 can get new logs, because it doesn't have `ToBlock` field
	s.Require().NoError(filters.process(&block, []*types.Receipt{receipt}))
	s.Empty(filter1.output)
	s.GreaterOrEqual(len(filter2.output), 1)
}

func TestFilters(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteFilters))
}
