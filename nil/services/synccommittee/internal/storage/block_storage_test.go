package storage

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/testaide"
	scTypes "github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	"github.com/stretchr/testify/suite"
)

type BlockStorageTestSuite struct {
	suite.Suite

	db           db.DB
	ctx          context.Context
	cancellation context.CancelFunc
	bs           BlockStorage
}

func (s *BlockStorageTestSuite) SetupSuite() {
	s.ctx, s.cancellation = context.WithCancel(context.Background())

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

func (s *BlockStorageTestSuite) TearDownSuite() {
	s.cancellation()
}

func TestBlockStorageTestSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(BlockStorageTestSuite))
}

func (s *BlockStorageTestSuite) TestGetSetBlock() {
	block := testaide.GenerateExecutionShardBlock(testaide.RandomHash())

	// Test SetBlock
	err := s.bs.SetBlock(s.ctx, block)
	s.Require().NoError(err)

	// Test GetBlock
	retrievedBlock, err := s.bs.GetBlock(s.ctx, scTypes.IdFromBlock(block))
	s.Require().NoError(err)
	s.Require().Equal(block.Number, retrievedBlock.Number)

	// Test GetBlock for non-existent block
	nonExistentBlock, err := s.bs.GetBlock(s.ctx, testaide.RandomBlockId())
	s.Require().NoError(err)
	s.Require().Nil(nonExistentBlock)
}

func (s *BlockStorageTestSuite) TestIsBatchCompleted() {
	mainBlock := testaide.GenerateExecutionShardBlock(testaide.RandomHash())
	mainBlock.ShardId = types.MainShardId
	mainBlock.ChildBlocks = make([]common.Hash, 1)

	err := s.bs.SetBlock(s.ctx, mainBlock)
	s.Require().NoError(err)

	block := testaide.GenerateExecutionShardBlock(testaide.RandomHash())
	block.ShardId = types.ShardId(1)
	block.MainChainHash = mainBlock.Hash

	mainBlock.ChildBlocks[0] = block.Hash

	err = s.bs.SetBlock(s.ctx, block)
	s.Require().NoError(err)

	res, err := s.bs.IsBatchCompleted(s.ctx, mainBlock)
	s.Require().NoError(err)
	s.Require().True(res)
}

func (s *BlockStorageTestSuite) TestGetSetBatchId() {
	block := testaide.GenerateMainShardBlock()

	// set BatchId
	err := s.bs.SetBlock(s.ctx, block)
	s.Require().NoError(err)

	// Test GetBatchId
	batchId, err := s.bs.GetBatchId(s.ctx, block)
	s.Require().NoError(err)
	s.Require().NotNil(batchId)
}

func (s *BlockStorageTestSuite) TestGetLastFetchedBlock() {
	shardId := types.ShardId(1)
	block1 := testaide.GenerateExecutionShardBlock(testaide.RandomHash())
	block1.ShardId = shardId
	block2 := testaide.GenerateExecutionShardBlock(testaide.RandomHash())
	block2.ShardId = shardId
	block2.Number = block1.Number + 1

	err := s.bs.SetBlock(s.ctx, block1)
	s.Require().NoError(err)
	err = s.bs.SetBlock(s.ctx, block2)
	s.Require().NoError(err)

	lastFetchedNum, err := s.bs.GetLatestFetched(s.ctx, shardId)
	s.Require().NoError(err)
	s.Require().Equal(block2.Number, lastFetchedNum)
}

func (s *BlockStorageTestSuite) TestSetBlockAsProved_DoesNotExist() {
	randomId := testaide.RandomBlockId()
	err := s.bs.SetBlockAsProved(s.ctx, randomId)
	s.Require().Errorf(err, "block with id=%s is not found", randomId.String())
}

func (s *BlockStorageTestSuite) TestSetBlockAsProved() {
	hash := testaide.RandomHash()

	for _, block := range []*jsonrpc.RPCBlock{
		{Number: 10, ShardId: types.MainShardId, ParentHash: testaide.RandomHash(), Hash: hash},
		{Number: 11, ShardId: types.MainShardId, ParentHash: testaide.RandomHash(), Hash: testaide.RandomHash()},
		{Number: 12, ShardId: types.MainShardId, ParentHash: testaide.RandomHash(), Hash: testaide.RandomHash()},
	} {
		err := s.bs.SetBlock(s.ctx, block)
		s.Require().NoError(err)
	}

	blockId := scTypes.NewBlockId(types.MainShardId, hash)
	err := s.bs.SetBlockAsProved(s.ctx, blockId)
	s.Require().NoError(err)
}

func (s *BlockStorageTestSuite) TestSetBlockAsProposed_DoesNotExist() {
	randomId := testaide.RandomBlockId()
	err := s.bs.SetBlockAsProposed(s.ctx, randomId)
	s.Require().Errorf(err, "block with id=%s is not found", randomId.String())
}

func (s *BlockStorageTestSuite) TestSetBlockAsProposed_IsNotProved() {
	block := testaide.GenerateMainShardBlock()
	err := s.bs.SetBlock(s.ctx, block)
	s.Require().NoError(err)

	err = s.bs.SetBlockAsProposed(s.ctx, scTypes.IdFromBlock(block))
	s.Require().Errorf(err, "block with hash=%s is not proved", block.Hash.String())
}

func (s *BlockStorageTestSuite) TestSetBlockAsProposed_ParentHashMismatch() {
	previousMainBlock := testaide.GenerateMainShardBlock()
	previousId := scTypes.IdFromBlock(previousMainBlock)

	err := s.bs.SetBlock(s.ctx, previousMainBlock)
	s.Require().NoError(err)

	err = s.bs.SetBlockAsProved(s.ctx, previousId)
	s.Require().NoError(err)

	err = s.bs.SetBlockAsProposed(s.ctx, previousId)
	s.Require().NoError(err)

	newMainBlock := testaide.GenerateMainShardBlock()
	newId := scTypes.IdFromBlock(newMainBlock)

	err = s.bs.SetBlock(s.ctx, newMainBlock)
	s.Require().NoError(err)

	err = s.bs.SetBlockAsProved(s.ctx, newId)
	s.Require().NoError(err)

	err = s.bs.SetBlockAsProposed(s.ctx, newId)
	s.Require().ErrorContains(err, "is not equal to the stored value")
}

func (s *BlockStorageTestSuite) TestSetBlockAsProposed_WithExecutionShardBlocks() {
	mainBlock := testaide.GenerateMainShardBlock()
	mainBlockId := scTypes.IdFromBlock(mainBlock)

	const blocksCount = 3
	executionShardBlocks := make([]*jsonrpc.RPCBlock, 0, blocksCount)
	for range blocksCount {
		nextBlock := testaide.GenerateExecutionShardBlock(mainBlock.Hash)
		executionShardBlocks = append(executionShardBlocks, nextBlock)
	}

	someOtherBlock := &jsonrpc.RPCBlock{
		Number: 15, ShardId: 3, Hash: testaide.RandomHash(), MainChainHash: testaide.RandomHash(),
	}

	for _, block := range append(executionShardBlocks, someOtherBlock, mainBlock) {
		err := s.bs.SetBlock(s.ctx, block)
		s.Require().NoError(err)
	}

	err := s.bs.SetBlockAsProved(s.ctx, mainBlockId)
	s.Require().NoError(err)

	err = s.bs.SetBlockAsProposed(s.ctx, mainBlockId)
	s.Require().NoError(err)

	for _, block := range append(executionShardBlocks, mainBlock) {
		blockFromDb, err := s.bs.GetBlock(s.ctx, scTypes.IdFromBlock(block))
		s.Require().NoError(err)
		s.Require().Nil(blockFromDb)
	}

	otherBlockFromDb, err := s.bs.GetBlock(s.ctx, scTypes.IdFromBlock(someOtherBlock))
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
	err = s.bs.SetBlock(s.ctx, mainBlock)
	s.Require().NoError(err)

	data, err := s.bs.TryGetNextProposalData(s.ctx)
	s.Require().Nil(data)
	s.Require().NoError(err)
}

func (s *BlockStorageTestSuite) TestTryGetNextProposalData_Collect_Transactions() {
	err := s.bs.SetProvedStateRoot(s.ctx, testaide.RandomHash())
	s.Require().NoError(err, "failed to set initial state root")

	var expectedTxCount int
	mainBlock := testaide.GenerateMainShardBlock()
	expectedTxCount += len(mainBlock.Messages)

	const blocksCount = 3
	executionShardBlocks := make([]*jsonrpc.RPCBlock, 0, blocksCount)

	for range blocksCount {
		block := testaide.GenerateExecutionShardBlock(mainBlock.Hash)
		executionShardBlocks = append(executionShardBlocks, block)
		expectedTxCount += len(block.Messages)
	}

	for _, block := range append(executionShardBlocks, mainBlock) {
		err := s.bs.SetBlock(s.ctx, block)
		s.Require().NoError(err)
	}

	err = s.bs.SetBlockAsProved(s.ctx, scTypes.IdFromBlock(mainBlock))
	s.Require().NoError(err)

	data, err := s.bs.TryGetNextProposalData(s.ctx)
	s.Require().NoError(err)
	s.Require().NotNil(data)
	s.Require().Len(data.Transactions, expectedTxCount)
}

func (s *BlockStorageTestSuite) TestTryGetNextProposalData_Concurrently() {
	initialStateRoot := testaide.RandomHash()
	err := s.bs.SetProvedStateRoot(s.ctx, initialStateRoot)
	s.Require().NoError(err, "failed to set initial state root")

	const blocksCount = 10
	mainShardBlocks := generateMainShardBlocks(blocksCount)

	for _, block := range mainShardBlocks {
		err := s.bs.SetBlock(s.ctx, &block)
		s.Require().NoError(err, "failed to set block")
	}

	waitGroup := sync.WaitGroup{}
	waitGroup.Add(blocksCount + 1)

	// concurrently set blocks as proved
	for _, block := range mainShardBlocks {
		go func() {
			err := s.bs.SetBlockAsProved(s.ctx, scTypes.IdFromBlock(&block))
			s.NoError(err, "failed to set block as proved")
			waitGroup.Done()
		}()
	}

	receiveTimeout := time.After(time.Second * 3)
	var receivedData []*ProposalData
	go func() {
		// poll all blocks data from the storage
		for {
			if len(receivedData) == blocksCount {
				break
			}
			select {
			case <-receiveTimeout:
				s.Fail("proposal data receive timeout exceeded")
				waitGroup.Done()
			default:
				data, err := s.bs.TryGetNextProposalData(s.ctx)
				s.NoError(err, "failed to get next proposal data")
				if data == nil {
					continue
				}

				receivedData = append(receivedData, data)
				blockId := scTypes.NewBlockId(types.MainShardId, data.MainShardBlockHash)
				err = s.bs.SetBlockAsProposed(s.ctx, blockId)
				s.NoError(err, "failed to set block as proposed")
			}
		}
		waitGroup.Done()
	}()

	waitGroup.Wait()

	msg := func(field string) string {
		return field + " is not equal to the expected value"
	}

	// check that data was received in correct order
	for idx := range blocksCount {
		block := mainShardBlocks[idx]
		data := receivedData[idx]

		s.Equal(block.Hash, data.MainShardBlockHash, msg("MainShardBlockHash"))
		s.Len(data.Transactions, len(block.Messages), msg("Transactions count"))
		s.Equal(block.ChildBlocksRootHash, data.NewProvedStateRoot, msg("NewProvedStateRoot"))

		if idx == 0 {
			s.Equal(initialStateRoot, data.OldProvedStateRoot, msg("OldProvedStateRoot"))
		} else {
			s.Equal(mainShardBlocks[idx-1].ChildBlocksRootHash, data.OldProvedStateRoot, msg("OldProvedStateRoot"))
		}
	}
}

func generateMainShardBlocks(blocksCount int) []jsonrpc.RPCBlock {
	mainShardBlocks := make([]jsonrpc.RPCBlock, 0, blocksCount)
	for range blocksCount {
		nextBlock := testaide.GenerateMainShardBlock()
		if len(mainShardBlocks) > 0 {
			nextBlock.ParentHash = mainShardBlocks[len(mainShardBlocks)-1].Hash
		} else {
			nextBlock.ParentHash = testaide.RandomHash()
		}
		mainShardBlocks = append(mainShardBlocks, *nextBlock)
	}
	return mainShardBlocks
}
