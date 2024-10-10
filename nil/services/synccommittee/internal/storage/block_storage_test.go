package storage

import (
	"context"
	"sync"
	"testing"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/testaide"
	"github.com/holiman/uint256"
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
	logger := logging.NewLogger("block_storage_test")
	s.bs = NewBlockStorage(s.db, logger)
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

func (s *BlockStorageTestSuite) TestSetBlockAsProved_DoesNotExist() {
	hash := testaide.RandomHash()
	err := s.bs.SetBlockAsProved(s.ctx, hash)
	s.Require().Errorf(err, "block with hash=%s is not found", hash.String())
}

func (s *BlockStorageTestSuite) TestSetBlockAsProved() {
	hash := testaide.RandomHash()

	for _, block := range []*jsonrpc.RPCBlock{
		{Number: 10, Hash: hash},
		{Number: 11, Hash: testaide.RandomHash()},
		{Number: 12, Hash: testaide.RandomHash()},
	} {
		err := s.bs.SetBlock(s.ctx, types.MainShardId, block.Number, block)
		s.Require().NoError(err)
	}

	err := s.bs.SetBlockAsProved(s.ctx, hash)
	s.Require().NoError(err)
}

func (s *BlockStorageTestSuite) TestSetBlockAsProposed_DoesNotExist() {
	hash := testaide.RandomHash()
	err := s.bs.SetBlockAsProposed(s.ctx, hash)
	s.Require().Errorf(err, "block with hash=%s is not found", hash.String())
}

func (s *BlockStorageTestSuite) TestSetBlockAsProposed_IsNotProved() {
	block := &jsonrpc.RPCBlock{Number: 10, Hash: testaide.RandomHash()}
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
	s.Require().ErrorContains(err, "is not equal to the stored value")
}

func (s *BlockStorageTestSuite) TestSetBlockAsProposed_WithExecutionShardBlocks() {
	mainBlock := testaide.GenerateMainShardBlock()

	executionShardBlocks := []*jsonrpc.RPCBlock{
		{Number: testaide.RandomBlockNum(), ShardId: 1, Hash: testaide.RandomHash(), MainChainHash: mainBlock.Hash},
		{Number: testaide.RandomBlockNum(), ShardId: 2, Hash: testaide.RandomHash(), MainChainHash: mainBlock.Hash},
		{Number: testaide.RandomBlockNum(), ShardId: 3, Hash: testaide.RandomHash(), MainChainHash: mainBlock.Hash},
	}

	someOtherBlock := &jsonrpc.RPCBlock{
		Number: 15, ShardId: 3, Hash: testaide.RandomHash(), MainChainHash: testaide.RandomHash(),
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

func (s *BlockStorageTestSuite) TestTryGetNextProposalData_NotInitializedStateRoot() {
	ctx := context.Background()
	data, err := s.bs.TryGetNextProposalData(ctx)
	s.Require().Nil(data)
	s.Require().Error(err, "proved state root was not initialized")
}

func (s *BlockStorageTestSuite) TestTryGetNextProposalData_BlockParentHashNotSet() {
	err := s.bs.SetProvedStateRoot(s.ctx, testaide.RandomHash())
	s.Require().NoError(err)

	data, err := s.bs.TryGetNextProposalData(s.ctx)
	s.Require().Nil(data)
	s.Require().NoError(err)
}

func (s *BlockStorageTestSuite) TestTryGetNextProposalData_NoProvedMainShardBlockFound() {
	err := s.bs.SetProvedStateRoot(s.ctx, testaide.RandomHash())
	s.Require().NoError(err)

	mainBlock := testaide.GenerateMainShardBlock()
	err = s.bs.SetBlock(s.ctx, mainBlock.ShardId, mainBlock.Number, mainBlock)
	s.Require().NoError(err)

	data, err := s.bs.TryGetNextProposalData(s.ctx)
	s.Require().Nil(data)
	s.Require().NoError(err)
}

func (s *BlockStorageTestSuite) TestTryGetNextProposalData_Concurrently() {
	initialStateRoot := testaide.RandomHash()
	err := s.bs.SetProvedStateRoot(s.ctx, initialStateRoot)
	s.Require().NoError(err)

	const blocksCount = 10

	var mainShardBlocks []jsonrpc.RPCBlock
	for range blocksCount {
		nextBlock := testaide.GenerateMainShardBlock()
		if len(mainShardBlocks) > 0 {
			nextBlock.ParentHash = mainShardBlocks[len(mainShardBlocks)-1].Hash
		}
	}

	for _, block := range mainShardBlocks {
		err := s.bs.SetBlock(s.ctx, block.ShardId, block.Number, &block)
		s.Require().NoError(err)
	}

	waitGroup := sync.WaitGroup{}
	waitGroup.Add(blocksCount + 1)

	// set blocks as proved in random order
	for _, block := range mainShardBlocks {
		go func() {
			err = s.bs.SetBlockAsProved(s.ctx, block.Hash)
			s.NoError(err)
			waitGroup.Done()
		}()
	}

	go func() {
		for idx, block := range mainShardBlocks {
			data, err := s.bs.TryGetNextProposalData(s.ctx)
			s.NoError(err)
			s.Equal(block.Hash, data.MainShardBlockHash)

			if idx == 0 {
				s.Equal(initialStateRoot, data.OldProvedStateRoot)
			} else {
				s.Equal(mainShardBlocks[idx-1].Hash, data.OldProvedStateRoot)
			}

			s.Equal(block.ChildBlocksRootHash, data.NewProvedStateRoot)
		}
		waitGroup.Done()
	}()

	waitGroup.Wait()
}

// todo: complete test implementation

func (s *BlockStorageTestSuite) TestTryGetNextProposalData_Collect_Transactions() {
	mainBlock := testaide.GenerateMainShardBlock()
	mainBlock.Messages = []any{
		jsonrpc.RPCInMessage{
			Flags: types.MessageFlags{},
			Seqno: 10,
			From:  types.EmptyAddress,
			To:    types.EmptyAddress,
			Value: types.NewValue(uint256.NewInt(1000)),
			Data:  []byte{10, 20, 30, 40},
		},
	}

	executionShardBlocks := []*jsonrpc.RPCBlock{
		{Number: testaide.RandomBlockNum(), ShardId: 1, Hash: testaide.RandomHash(), MainChainHash: mainBlock.Hash},
		{Number: testaide.RandomBlockNum(), ShardId: 2, Hash: testaide.RandomHash(), MainChainHash: mainBlock.Hash},
		{Number: testaide.RandomBlockNum(), ShardId: 3, Hash: testaide.RandomHash(), MainChainHash: mainBlock.Hash},
	}

	for _, block := range append(executionShardBlocks, mainBlock) {
		err := s.bs.SetBlock(s.ctx, block.ShardId, block.Number, block)
		s.Require().NoError(err)
	}

	err := s.bs.SetBlockAsProved(s.ctx, mainBlock.Hash)
	s.Require().NoError(err)

	data, err := s.bs.TryGetNextProposalData(s.ctx)
	s.Require().NoError(err)
	s.Require().NotNil(data)
	s.Require().Len(data.Transactions, 0)
}
