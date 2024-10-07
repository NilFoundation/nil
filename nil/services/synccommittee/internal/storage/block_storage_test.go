package storage

import (
	"context"
	"testing"

	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/testaide"
	"github.com/stretchr/testify/suite"
)

type BlockStorageTestSuite struct {
	suite.Suite

	db  db.DB
	ctx context.Context
	bs  BlockStorage
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

func (s *BlockStorageTestSuite) TestSetBlockAsProved() {
	hash := testaide.GenerateRandomBlockHash()

	for _, block := range []*jsonrpc.RPCBlock{
		{Number: 10, Hash: hash},
		{Number: 11, Hash: testaide.GenerateRandomBlockHash()},
		{Number: 12, Hash: testaide.GenerateRandomBlockHash()},
	} {
		err := s.bs.SetBlock(s.ctx, types.MainShardId, block.Number, block)
		s.Require().NoError(err)
	}

	err := s.bs.SetBlockAsProved(s.ctx, hash)
	s.Require().NoError(err)
}

func TestBlockStorageTestSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(BlockStorageTestSuite))
}
