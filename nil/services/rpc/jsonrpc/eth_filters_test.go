package jsonrpc

import (
	"context"
	"testing"
	"time"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/filters"
	"github.com/stretchr/testify/suite"
)

type SuiteEthFilters struct {
	suite.Suite
	ctx     context.Context
	cancel  context.CancelFunc
	db      db.DB
	api     *APIImpl
	shardId types.ShardId
}

const (
	ManagerWaitTimeout  = 2000 * time.Millisecond
	ManagerPollInterval = 200 * time.Millisecond
)

func (s *SuiteEthFilters) SetupTest() {
	s.ctx, s.cancel = context.WithCancel(context.Background())
	var err error
	s.db, err = db.NewBadgerDbInMemory()
	s.Require().NoError(err)

	s.api = NewTestEthAPI(s.ctx, s.T(), s.db, 1)
}

func (s *SuiteEthFilters) TearDownTest() {
	s.cancel()
	s.db.Close()
}

func (s *SuiteEthFilters) TestLogs() {
	tx, err := s.db.CreateRwTx(s.ctx)
	s.Require().NoError(err)
	defer tx.Rollback()

	address1 := types.HexToAddress("0x1111111111")
	address2 := types.HexToAddress("0x2222222222")

	topics := [][]common.Hash{{}, {}, {{3}}}
	query1 := filters.FilterQuery{
		Addresses: []types.Address{address1},
		Topics:    topics,
	}
	id1, err := s.api.NewFilter(s.ctx, query1)
	s.Require().NoError(err)
	s.Require().NotEmpty(id1)

	topics2 := [][]common.Hash{{}, {{2}}}
	query2 := filters.FilterQuery{
		Addresses: []types.Address{},
		Topics:    topics2,
	}
	id2, err := s.api.NewFilter(s.ctx, query2)
	s.Require().NoError(err)
	s.Require().NotEmpty(id2)

	logsInput := []*types.Log{
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
	logsInput2 := []*types.Log{
		{
			Address: address2,
			Topics:  []common.Hash{{0x03}, {0x02}},
			Data:    []byte{0xaa, 0xaa},
		},
	}

	receiptsMpt := execution.NewDbReceiptTrie(tx, s.shardId)
	s.Require().NoError(receiptsMpt.Update(0, &types.Receipt{ContractAddress: address1, Logs: logsInput}))
	s.Require().NoError(receiptsMpt.Update(1, &types.Receipt{ContractAddress: address2, Logs: logsInput2}))

	block := types.Block{
		BlockData: types.BlockData{
			ReceiptsRoot: receiptsMpt.RootHash(),
		},
	}
	blockHash := block.Hash(s.shardId)
	s.Require().NoError(db.WriteBlock(tx, s.shardId, blockHash, &block))
	s.Require().NoError(db.WriteLastBlockHash(tx, types.MainShardId, blockHash))
	s.Require().NoError(tx.Commit())

	var logs []any
	s.Require().Eventually(func() bool {
		logs, err = s.api.GetFilterChanges(s.ctx, id1)
		s.Require().NoError(err)
		return len(logs) == 2
	}, ManagerWaitTimeout, ManagerPollInterval)

	log0, ok := logs[0].(*RPCLog)
	s.Require().True(ok)
	log1, ok := logs[1].(*RPCLog)
	s.Require().True(ok)

	s.Require().Equal(logsInput[0].Data, log0.Data)
	s.Require().Equal(logsInput[3].Data, log1.Data)

	logs, err = s.api.GetFilterChanges(s.ctx, id2)

	log0, ok = logs[0].(*RPCLog)
	s.Require().True(ok)
	log1, ok = logs[1].(*RPCLog)
	s.Require().True(ok)

	s.Require().NoError(err)
	s.Require().Len(logs, 2)
	s.Require().Equal(logsInput[0].Data, log0.Data)
	s.Require().Equal(logsInput2[0].Data, log1.Data)
}

func (s *SuiteEthFilters) TestBlocks() {
	tx, err := s.db.CreateRwTx(s.ctx)
	s.Require().NoError(err)
	defer tx.Rollback()
	shardId := types.ShardId(0)

	id1, err := s.api.NewBlockFilter(s.ctx)
	s.Require().NoError(err)
	s.Require().NotEmpty(id1)

	// No blocks should be
	blocks, err := s.api.GetFilterChanges(s.ctx, id1)
	s.Require().NoError(err)
	s.Require().Empty(blocks)

	block1 := types.Block{BlockData: types.BlockData{Id: 1}}

	// Add one block
	blockHash := block1.Hash(shardId)
	s.Require().NoError(db.WriteBlock(tx, shardId, blockHash, &block1))
	s.Require().NoError(db.WriteLastBlockHash(tx, types.MainShardId, blockHash))
	s.Require().NoError(tx.Commit())

	// id1 filter should see 1 block
	s.Require().Eventually(func() bool {
		blocks, err = s.api.GetFilterChanges(s.ctx, id1)
		s.Require().NoError(err)
		return len(blocks) == 1
	}, ManagerWaitTimeout, ManagerPollInterval)

	s.Require().IsType(&types.Block{}, blocks[0])
	block, ok := blocks[0].(*types.Block)
	s.Require().True(ok)
	s.Require().Equal(block.Id, block1.Id)

	// Add block filter id2
	id2, err := s.api.NewBlockFilter(s.ctx)
	s.Require().NoError(err)
	s.Require().NotEmpty(id2)

	tx, err = s.db.CreateRwTx(s.ctx)
	s.Require().NoError(err)
	defer tx.Rollback()

	// Add new three blocks
	block2 := types.Block{BlockData: types.BlockData{Id: 2, PrevBlock: block1.Hash(shardId)}}
	block3 := types.Block{BlockData: types.BlockData{Id: 3, PrevBlock: block2.Hash(shardId)}}
	block4 := types.Block{BlockData: types.BlockData{Id: 4, PrevBlock: block3.Hash(shardId)}}
	s.Require().NoError(db.WriteBlock(tx, shardId, block2.Hash(shardId), &block2))
	s.Require().NoError(db.WriteBlock(tx, shardId, block3.Hash(shardId), &block3))
	s.Require().NoError(db.WriteBlock(tx, shardId, block4.Hash(shardId), &block4))
	s.Require().NoError(db.WriteLastBlockHash(tx, types.MainShardId, block4.Hash(types.MainShardId)))
	s.Require().NoError(tx.Commit())

	// Both filters should see these blocks
	s.Require().Eventually(func() bool {
		for _, id := range []string{id1, id2} {
			blocks, err = s.api.GetFilterChanges(s.ctx, id)
			s.Require().NoError(err)
			if len(blocks) != 3 {
				return false
			}
			s.Require().Len(blocks, 3)
			block, ok = blocks[0].(*types.Block)
			s.Require().True(ok)
			s.Require().Equal(block.Id, block4.Id)
			block, ok = blocks[1].(*types.Block)
			s.Require().True(ok)
			s.Require().Equal(block.Id, block3.Id)
			block, ok = blocks[2].(*types.Block)
			s.Require().True(ok)
			s.Require().Equal(block.Id, block2.Id)
		}
		return true
	}, ManagerWaitTimeout, ManagerPollInterval)

	// Uninstall id1 block filter
	deleted, err := s.api.UninstallFilter(s.ctx, id1)
	s.Require().True(deleted)
	s.Require().NoError(err)

	// Uninstall second time should return error
	deleted, err = s.api.UninstallFilter(s.ctx, id1)
	s.Require().False(deleted)
	s.Require().NoError(err)

	tx, err = s.db.CreateRwTx(s.ctx)
	s.Require().NoError(err)
	defer tx.Rollback()

	// Add another two blocks
	block5 := types.Block{BlockData: types.BlockData{Id: 5, PrevBlock: block4.Hash(types.MainShardId)}}
	block6 := types.Block{BlockData: types.BlockData{Id: 6, PrevBlock: block5.Hash(shardId)}}
	s.Require().NoError(db.WriteBlock(tx, shardId, block5.Hash(shardId), &block5))
	s.Require().NoError(db.WriteBlock(tx, shardId, block6.Hash(types.MainShardId), &block6))
	s.Require().NoError(db.WriteLastBlockHash(tx, types.MainShardId, block6.Hash(types.MainShardId)))
	s.Require().NoError(tx.Commit())

	// id1 is deleted, expect error
	s.Require().Eventually(func() bool {
		blocks, err = s.api.GetFilterChanges(s.ctx, id1)
		s.Require().Error(err)
		return len(blocks) == 0
	}, ManagerWaitTimeout, ManagerPollInterval)

	// Expect two blocks for id2
	s.Require().Eventually(func() bool {
		blocks, err = s.api.GetFilterChanges(s.ctx, id2)
		s.Require().NoError(err)
		return len(blocks) == 2
	}, ManagerWaitTimeout, ManagerPollInterval)

	block, ok = blocks[0].(*types.Block)
	s.Require().True(ok)
	s.Require().Equal(block.Id, block6.Id)
	block, ok = blocks[1].(*types.Block)
	s.Require().True(ok)
	s.Require().Equal(block.Id, block5.Id)

	// Uninstall second filter
	deleted, err = s.api.UninstallFilter(s.ctx, id2)
	s.Require().True(deleted)
	s.Require().NoError(err)
}

func TestEthFilters(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteEthFilters))
}
