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
	err := s.bs.SetBlock(s.ctx, block, block.Hash)
	s.Require().NoError(err)

	// Test TryGetBlock
	retrievedBlock, err := s.bs.TryGetBlock(s.ctx, scTypes.IdFromBlock(block))
	s.Require().NoError(err)
	s.Require().Equal(block.Number, retrievedBlock.Number)

	// Test TryGetBlock for non-existent block
	nonExistentBlock, err := s.bs.TryGetBlock(s.ctx, testaide.RandomBlockId())
	s.Require().NoError(err)
	s.Require().Nil(nonExistentBlock)
}

func (s *BlockStorageTestSuite) TestSetBlockSequentially_GetConcurrently() {
	const blocksCount = 5
	blocks := testaide.GenerateMainShardBlocks(blocksCount)

	for _, block := range blocks {
		err := s.bs.SetBlock(s.ctx, block, block.Hash)
		s.Require().NoError(err)
	}

	waitGroup := sync.WaitGroup{}
	waitGroup.Add(blocksCount)

	for _, block := range blocks {
		go func() {
			blockId := scTypes.IdFromBlock(block)
			fromDb, err := s.bs.TryGetBlock(s.ctx, blockId)
			s.NoError(err)
			s.NotNil(fromDb)
			s.Equal(block.Number, fromDb.Number)
			s.Equal(block.Hash, fromDb.Hash)
			waitGroup.Done()
		}()
	}

	waitGroup.Wait()
}

func (s *BlockStorageTestSuite) TestIsBatchCompleted() {
	mainBlock := testaide.GenerateExecutionShardBlock(testaide.RandomHash())
	mainBlock.ShardId = types.MainShardId
	mainBlock.ChildBlocks = make([]common.Hash, 1)

	err := s.bs.SetBlock(s.ctx, mainBlock, mainBlock.Hash)
	s.Require().NoError(err)

	block := testaide.GenerateExecutionShardBlock(testaide.RandomHash())
	block.ShardId = types.ShardId(1)
	block.MainChainHash = mainBlock.Hash

	mainBlock.ChildBlocks[0] = block.Hash

	err = s.bs.SetBlock(s.ctx, block, mainBlock.Hash)
	s.Require().NoError(err)

	res, err := s.bs.IsBatchCompleted(s.ctx, mainBlock)
	s.Require().NoError(err)
	s.Require().True(res)
}

func (s *BlockStorageTestSuite) TestGetOrCreateBatchId() {
	// test create new batch Id
	mainBlockHash := testaide.RandomHash()
	expectedBatchId, err := s.bs.GetOrCreateBatchId(s.ctx, mainBlockHash)
	s.Require().NoError(err)

	// test get batch Id
	batchId, err := s.bs.GetOrCreateBatchId(s.ctx, mainBlockHash)
	s.Require().NoError(err)
	s.Require().Equal(expectedBatchId, batchId)
}

func (s *BlockStorageTestSuite) TestGetLastFetchedBlock() {
	// initially latestFetched should be empty
	latestFetched, err := s.bs.TryGetLatestFetched(s.ctx)
	s.Require().NoError(err)
	s.Require().Nil(latestFetched)

	mainBlock := testaide.GenerateMainShardBlock()
	execBlocks := testaide.GenerateExecutionShardBlocks(mainBlock.Hash, 10)

	// blocks from different execution shards can be saved in any order
	waitGroup := sync.WaitGroup{}
	waitGroup.Add(len(execBlocks))
	for _, block := range execBlocks {
		go func() {
			err := s.bs.SetBlock(s.ctx, block, mainBlock.Hash)
			s.NoError(err)
			waitGroup.Done()
		}()
	}
	waitGroup.Wait()

	// latest fetched should not be affected by execution blocks
	latestFetched, err = s.bs.TryGetLatestFetched(s.ctx)
	s.Require().NoError(err)
	s.Require().Nil(latestFetched)

	err = s.bs.SetBlock(s.ctx, mainBlock, mainBlock.Hash)
	s.Require().NoError(err)

	// latestFetched is updated after the main shard block is saved
	latestFetched, err = s.bs.TryGetLatestFetched(s.ctx)
	s.Require().NoError(err)
	s.Require().NotNil(latestFetched)
	s.Require().Equal(mainBlock.Number, latestFetched.Number)
	s.Require().Equal(mainBlock.Hash, latestFetched.Hash)
}

func (s *BlockStorageTestSuite) TestSetBlockAsProved_DoesNotExist() {
	randomId := testaide.RandomBlockId()
	err := s.bs.SetBlockAsProved(s.ctx, randomId)
	s.Require().Errorf(err, "block with id=%s is not found", randomId.String())
}

func (s *BlockStorageTestSuite) TestSetBlockAsProved() {
	block := testaide.GenerateMainShardBlock()

	err := s.bs.SetBlock(s.ctx, block, block.Hash)
	s.Require().NoError(err)

	err = s.bs.SetBlockAsProved(s.ctx, scTypes.IdFromBlock(block))
	s.Require().NoError(err)
}

func (s *BlockStorageTestSuite) TestSetBlockAsProposed_DoesNotExist() {
	randomId := testaide.RandomBlockId()
	err := s.bs.SetBlockAsProposed(s.ctx, randomId)
	s.Require().Errorf(err, "block with id=%s is not found", randomId.String())
}

func (s *BlockStorageTestSuite) TestSetBlockAsProposed_IsNotProved() {
	block := testaide.GenerateMainShardBlock()
	err := s.bs.SetBlock(s.ctx, block, block.Hash)
	s.Require().NoError(err)

	err = s.bs.SetBlockAsProposed(s.ctx, scTypes.IdFromBlock(block))
	s.Require().Errorf(err, "block with hash=%s is not proved", block.Hash.String())
}

func (s *BlockStorageTestSuite) TestSetBlock_ParentHashMismatch() {
	previousMainBlock := testaide.GenerateMainShardBlock()
	previousId := scTypes.IdFromBlock(previousMainBlock)

	err := s.bs.SetBlock(s.ctx, previousMainBlock, previousMainBlock.Hash)
	s.Require().NoError(err)

	err = s.bs.SetBlockAsProved(s.ctx, previousId)
	s.Require().NoError(err)

	err = s.bs.SetBlockAsProposed(s.ctx, previousId)
	s.Require().NoError(err)

	newMainBlock := testaide.GenerateMainShardBlock()
	newMainBlock.Number = previousMainBlock.Number + 1

	err = s.bs.SetBlock(s.ctx, newMainBlock, newMainBlock.Hash)
	s.Require().ErrorContains(err, "unable to update latest fetched block: block mismatch")
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

	for _, block := range append(executionShardBlocks, mainBlock) {
		err := s.bs.SetBlock(s.ctx, block, mainBlock.Hash)
		s.Require().NoError(err)
	}
	err := s.bs.SetBlock(s.ctx, someOtherBlock, someOtherBlock.MainChainHash)
	s.Require().NoError(err)

	err = s.bs.SetBlockAsProved(s.ctx, mainBlockId)
	s.Require().NoError(err)

	err = s.bs.SetBlockAsProposed(s.ctx, mainBlockId)
	s.Require().NoError(err)

	for _, block := range append(executionShardBlocks, mainBlock) {
		blockFromDb, err := s.bs.TryGetBlock(s.ctx, scTypes.IdFromBlock(block))
		s.Require().NoError(err)
		s.Require().Nil(blockFromDb)
	}

	otherBlockFromDb, err := s.bs.TryGetBlock(s.ctx, scTypes.IdFromBlock(someOtherBlock))
	s.Require().NoError(err)
	s.Require().NotNil(otherBlockFromDb)
}

func (s *BlockStorageTestSuite) TestTryGetNextProposalData_NotInitializedStateRoot() {
	data, err := s.bs.TryGetNextProposalData(s.ctx)
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
	err = s.bs.SetBlock(s.ctx, mainBlock, mainBlock.Hash)
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
		err := s.bs.SetBlock(s.ctx, block, mainBlock.Hash)
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
	mainShardBlocks := testaide.GenerateMainShardBlocks(blocksCount)

	for _, block := range mainShardBlocks {
		err := s.bs.SetBlock(s.ctx, block, block.Hash)
		s.Require().NoError(err, "failed to set block")
	}

	waitGroup := sync.WaitGroup{}
	waitGroup.Add(blocksCount + 1)

	// concurrently set blocks as proved
	for _, block := range mainShardBlocks {
		go func() {
			err := s.bs.SetBlockAsProved(s.ctx, scTypes.IdFromBlock(block))
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
