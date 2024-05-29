package jsonrpc

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/mpt"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/msgpool"
	"github.com/NilFoundation/nil/rpc/filters"
	"github.com/NilFoundation/nil/rpc/transport/rpccfg"
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

func (s *SuiteEthFilters) SetupTest() {
	s.ctx, s.cancel = context.WithCancel(context.Background())
	var err error
	s.db, err = db.NewBadgerDbInMemory()
	s.Require().NoError(err)

	pool := msgpool.New(msgpool.DefaultConfig)
	s.Require().NotNil(pool)

	s.api = NewEthAPI(s.ctx,
		NewBaseApi(rpccfg.DefaultEvmCallTimeout), s.db, []msgpool.Pool{pool}, common.NewLogger("Test"))
}

func (s *SuiteEthFilters) TearDownTest() {
	s.cancel()
	s.db.Close()
}

func (s *SuiteEthFilters) TestLogs() {
	tx, err := s.db.CreateRwTx(s.ctx)
	s.Require().NoError(err)
	address1 := common.HexToAddress("0x1111111111")
	address2 := common.HexToAddress("0x2222222222")

	topics := [][]common.Hash{{}, {}, {{3}}}
	query1 := filters.FilterQuery{
		BlockHash: nil,
		FromBlock: nil,
		ToBlock:   nil,
		Addresses: []common.Address{address1},
		Topics:    topics,
	}
	id1, err := s.api.NewFilter(s.ctx, query1)
	s.Require().NoError(err)
	s.Require().NotEmpty(id1)

	topics2 := [][]common.Hash{{}, {{2}}}
	query2 := filters.FilterQuery{
		BlockHash: nil,
		FromBlock: nil,
		ToBlock:   nil,
		Addresses: []common.Address{},
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
	receiptsMpt := mpt.NewMerklePatriciaTrie(s.db, s.shardId, db.ReceiptTrieTable)

	receipt := &types.Receipt{ContractAddress: address1, Logs: logsInput}
	receiptEncoded, err := receipt.MarshalSSZ()
	s.Require().NoError(err)
	key, err := receipt.HashTreeRoot()
	s.Require().NoError(err)
	s.Require().NoError(receiptsMpt.Set(key[:], receiptEncoded))

	receipt2 := &types.Receipt{ContractAddress: address2, Logs: logsInput2}
	receiptEncoded, err = receipt2.MarshalSSZ()
	s.Require().NoError(err)
	key, err = receipt2.HashTreeRoot()
	s.Require().NoError(err)
	s.Require().NoError(receiptsMpt.Set(key[:], receiptEncoded))

	block := types.Block{
		ReceiptsRoot: receiptsMpt.RootHash(),
	}

	s.Require().NoError(db.WriteBlock(tx, s.shardId, &block))
	s.Require().NoError(tx.Put(db.LastBlockTable, types.MasterShardId.Bytes(), block.Hash().Bytes()))
	s.Require().NoError(tx.Commit())

	// Wait a bit so the filters detect new block
	time.Sleep(500 * time.Millisecond)

	logs, err := s.api.GetFilterChanges(s.ctx, id1)
	s.Require().NoError(err)
	s.Require().Len(logs, 2)
	s.Require().Equal(logsInput[0], logs[0])
	s.Require().Equal(logsInput[3], logs[1])

	logs, err = s.api.GetFilterChanges(s.ctx, id2)
	s.Require().NoError(err)
	s.Require().Len(logs, 2)
	s.Require().Equal(logsInput[0], logs[1])
	s.Require().Equal(logsInput2[0], logs[0])
}

func (s *SuiteEthFilters) TestBlocks() {
	tx, err := s.db.CreateRwTx(s.ctx)
	s.Require().NoError(err)
	shardId := types.ShardId(0)

	id1, err := s.api.NewBlockFilter(s.ctx)
	s.Require().NoError(err)
	s.Require().NotEmpty(id1)

	// No blocks should be
	blocks, err := s.api.GetFilterChanges(s.ctx, id1)
	s.Require().NoError(err)
	s.Require().Empty(blocks)

	block1 := types.Block{Id: 1}

	// Add one block
	s.Require().NoError(db.WriteBlock(tx, shardId, &block1))
	s.Require().NoError(tx.Put(db.LastBlockTable, []byte(strconv.Itoa(0)), block1.Hash().Bytes()))
	s.Require().NoError(tx.Commit())

	// Wait some time, so filters manager processes new blocks
	time.Sleep(200 * time.Millisecond)

	// id1 filter should see 1 block
	blocks, err = s.api.GetFilterChanges(s.ctx, id1)
	s.Require().NoError(err)
	s.Require().Len(blocks, 1)
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

	// Add new three blocks
	block2 := types.Block{Id: 2, PrevBlock: block1.Hash()}
	block3 := types.Block{Id: 3, PrevBlock: block2.Hash()}
	block4 := types.Block{Id: 4, PrevBlock: block3.Hash()}
	s.Require().NoError(db.WriteBlock(tx, shardId, &block2))
	s.Require().NoError(db.WriteBlock(tx, shardId, &block3))
	s.Require().NoError(db.WriteBlock(tx, shardId, &block4))
	s.Require().NoError(tx.Put(db.LastBlockTable, []byte(strconv.Itoa(0)), block4.Hash().Bytes()))
	s.Require().NoError(tx.Commit())

	// Wait some time, so filters manager processes new blocks
	time.Sleep(200 * time.Millisecond)

	// Both filters should see these blocks
	for _, id := range []string{id1, id2} {
		blocks, err = s.api.GetFilterChanges(s.ctx, id)
		s.Require().NoError(err)
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

	// Add another two blocks
	block5 := types.Block{Id: 5, PrevBlock: block4.Hash()}
	block6 := types.Block{Id: 6, PrevBlock: block5.Hash()}
	s.Require().NoError(db.WriteBlock(tx, shardId, &block5))
	s.Require().NoError(db.WriteBlock(tx, shardId, &block6))
	s.Require().NoError(tx.Put(db.LastBlockTable, []byte(strconv.Itoa(0)), block6.Hash().Bytes()))
	s.Require().NoError(tx.Commit())

	// Wait some time, so filters manager processes new blocks
	time.Sleep(200 * time.Millisecond)

	// id1 is deleted, expect error
	blocks, err = s.api.GetFilterChanges(s.ctx, id1)
	s.Require().Error(err)
	s.Require().Empty(blocks)

	// Expect two blocks for id2
	blocks, err = s.api.GetFilterChanges(s.ctx, id2)
	s.Require().NoError(err)
	s.Require().Len(blocks, 2)
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
