package synccommittee

import (
	"context"
	"testing"

	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
	"github.com/stretchr/testify/suite"
)

type BlockStorageTestSuite struct {
	suite.Suite

	db  db.DB
	ctx context.Context
	bs  *BlockStorage
}

func (s *BlockStorageTestSuite) SetupSuite() {
	var err error
	s.db, err = db.NewBadgerDbInMemory()
	s.Require().NoError(err)
	s.bs = NewBlockStorage(s.db)
}

func (s *BlockStorageTestSuite) SetupTest() {
	err := s.db.DropAll()
	s.Require().NoError(err)
}

func (s *BlockStorageTestSuite) TestGetSetBlock() {
	shardId := types.ShardId(1)
	blockNumber := types.BlockNumber(10)
	block := &jsonrpc.RPCBlock{Number: blockNumber}

	// Test SetBlock
	err := s.bs.SetBlock(s.ctx, shardId, blockNumber, block)
	s.Require().NoError(err)

	// Test GetBlock
	retrievedBlock, err := s.bs.GetBlock(s.ctx, shardId, blockNumber)
	s.Require().NoError(err)
	s.Require().Equal(block.Number, retrievedBlock.Number)

	// Test GetBlock for non-existent block
	nonExistentBlock, err := s.bs.GetBlock(s.ctx, shardId, blockNumber+1)
	s.Require().NoError(err)
	s.Require().Nil(nonExistentBlock)
}

func (s *BlockStorageTestSuite) TestGetLastFetchedBlock() {
	shardId := types.ShardId(1)
	block1 := &jsonrpc.RPCBlock{Number: 10}
	block2 := &jsonrpc.RPCBlock{Number: 20}

	err := s.bs.SetBlock(s.ctx, shardId, block1.Number, block1)
	s.Require().NoError(err)
	err = s.bs.SetBlock(s.ctx, shardId, block2.Number, block2)
	s.Require().NoError(err)

	lastFetchedNum, err := s.bs.GetLastFetchedBlockNum(s.ctx, shardId)
	s.Require().NoError(err)
	s.Require().Equal(block2.Number, lastFetchedNum)
}

func (s *BlockStorageTestSuite) TestGetSetLastProvedBlockNum() {
	shardId := types.ShardId(1)
	blockNum := types.BlockNumber(100)

	err := s.bs.SetLastProvedBlockNum(s.ctx, shardId, blockNum)
	s.Require().NoError(err)
	lastProved, err := s.bs.GetLastProvedBlockNum(s.ctx, shardId)
	s.Require().NoError(err)
	s.Require().Equal(blockNum, lastProved)
}

func (s *BlockStorageTestSuite) TestGetBlocksRange() {
	shardId := types.ShardId(1)
	for i := types.BlockNumber(1); i <= 10; i++ {
		err := s.bs.SetBlock(s.ctx, shardId, i, &jsonrpc.RPCBlock{Number: i})
		s.Require().NoError(err)
	}

	blocks, err := s.bs.GetBlocksRange(s.ctx, shardId, 3, 8)
	s.Require().NoError(err)
	s.Require().Len(blocks, 5)

	for i, block := range blocks {
		expectedNumber := types.BlockNumber(i + 3)
		s.Require().Equal(expectedNumber, block.Number)
	}

	// Test empty range
	emptyBlocks, err := s.bs.GetBlocksRange(s.ctx, shardId, 11, 15)
	s.Require().NoError(err)
	s.Require().Empty(emptyBlocks)
}

func (s *BlockStorageTestSuite) TestCleanupStorage() {
	const shardId = types.ShardId(1)
	const totalBlocks = 10
	for i := range types.BlockNumber(totalBlocks) {
		err := s.bs.SetBlock(s.ctx, shardId, i, &jsonrpc.RPCBlock{Number: i})
		s.Require().NoError(err)
	}

	const lastProvedBlkNum = 5
	err := s.bs.SetLastProvedBlockNum(s.ctx, shardId, lastProvedBlkNum)
	s.Require().NoError(err)
	err = s.bs.CleanupStorage(s.ctx)
	s.Require().NoError(err)

	for blkNum := range types.BlockNumber(totalBlocks) {
		block, err := s.bs.GetBlock(s.ctx, shardId, blkNum)
		s.Require().NoError(err)
		if blkNum < 5 && block != nil {
			s.Failf("Block left after cleanup", "blkNum: %d", blkNum)
		} else if blkNum >= 5 && block == nil {
			s.Failf("Block should not have been cleaned up, but it doesn't exist", "blkNum: %d", blkNum)
		}
	}
}

func TestBlockStorageTestSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(BlockStorageTestSuite))
}
