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

func TestBlockStorageTestSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(BlockStorageTestSuite))
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
	hash := testaide.RandomBlockHash()

	for _, block := range []*jsonrpc.RPCBlock{
		{Number: 10, Hash: hash},
		{Number: 11, Hash: testaide.RandomBlockHash()},
		{Number: 12, Hash: testaide.RandomBlockHash()},
	} {
		err := s.bs.SetBlock(s.ctx, types.MainShardId, block.Number, block)
		s.Require().NoError(err)
	}

	err := s.bs.SetBlockAsProved(s.ctx, hash)
	s.Require().NoError(err)
}

func (s *BlockStorageTestSuite) TestSetBlockAsProposed_DoesNotExist() {
	hash := testaide.RandomBlockHash()
	err := s.bs.SetBlockAsProposed(s.ctx, hash)
	s.Require().Errorf(err, "block with hash=%s is not found", hash.String())
}

func (s *BlockStorageTestSuite) TestSetBlockAsProposed_IsNotProved() {
	block := &jsonrpc.RPCBlock{Number: 10, Hash: testaide.RandomBlockHash()}
	err := s.bs.SetBlock(s.ctx, types.MainShardId, block.Number, block)
	s.Require().NoError(err)

	err = s.bs.SetBlockAsProposed(s.ctx, block.Hash)
	s.Require().Errorf(err, "block with hash=%s is not proved", block.Hash.String())
}

func (s *BlockStorageTestSuite) TestSetBlockAsProposed_ParentHashMismatch() {
	previousMainBlock := testaide.GenerateMainShardBlock()
	err := s.bs.SetBlock(s.ctx, previousMainBlock.ShardId, previousMainBlock.Number, previousMainBlock)
	err = s.bs.SetBlockAsProved(s.ctx, previousMainBlock.Hash)
	s.Require().NoError(err)

	err = s.bs.SetBlockAsProposed(s.ctx, previousMainBlock.Hash)
	s.Require().NoError(err)

	newMainBlock := testaide.GenerateMainShardBlock()
	err = s.bs.SetBlock(s.ctx, newMainBlock.ShardId, newMainBlock.Number, newMainBlock)
	err = s.bs.SetBlockAsProved(s.ctx, newMainBlock.Hash)
	s.Require().NoError(err)

	err = s.bs.SetBlockAsProposed(s.ctx, newMainBlock.Hash)
	s.Require().ErrorContains(err, "is not equal to the parent's block hash")
}

func (s *BlockStorageTestSuite) TestSetBlockAsProposed_WithExecutionShardBlocks() {
	mainBlock := testaide.GenerateMainShardBlock()

	executionShardBlocks := []*jsonrpc.RPCBlock{
		{Number: testaide.RandomBlockNum(), ShardId: 1, Hash: testaide.RandomBlockHash(), MainChainHash: mainBlock.Hash},
		{Number: testaide.RandomBlockNum(), ShardId: 2, Hash: testaide.RandomBlockHash(), MainChainHash: mainBlock.Hash},
		{Number: testaide.RandomBlockNum(), ShardId: 3, Hash: testaide.RandomBlockHash(), MainChainHash: mainBlock.Hash},
	}

	someOtherBlock := &jsonrpc.RPCBlock{
		Number: 15, ShardId: 3, Hash: testaide.RandomBlockHash(), MainChainHash: testaide.RandomBlockHash(),
	}

	for _, block := range append(executionShardBlocks, someOtherBlock, mainBlock) {
		err := s.bs.SetBlock(s.ctx, block.ShardId, block.Number, block)
		s.Require().NoError(err)
	}

	err := s.bs.SetBlockAsProved(s.ctx, mainBlock.Hash)
	s.Require().NoError(err)

	err = s.bs.SetBlockAsProposed(s.ctx, mainBlock.Hash)
	s.Require().NoError(err)

	for _, block := range append(executionShardBlocks, mainBlock) {
		blockFromDb, _ := s.bs.GetBlock(s.ctx, block.ShardId, block.Number)
		s.Require().Nil(blockFromDb)
	}

	otherBlockFromDb, err := s.bs.GetBlock(s.ctx, someOtherBlock.ShardId, someOtherBlock.Number)
	s.Require().NoError(err)
	s.Require().NotNil(otherBlockFromDb)
}
