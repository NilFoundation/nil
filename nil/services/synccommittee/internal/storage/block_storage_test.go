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
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/metrics"
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
	metricsHandler, err := metrics.NewSyncCommitteeMetrics()
	s.Require().NoError(err)

	timer := common.NewTimer()
	s.bs = NewBlockStorage(s.db, timer, metricsHandler, logger)
}

func (s *BlockStorageTestSuite) SetupTest() {
	err := s.db.DropAll()
	s.Require().NoError(err, "failed to clear storage in SetupTest")
}

func (s *BlockStorageTestSuite) SetupSubTest() {
	err := s.db.DropAll()
	s.Require().NoError(err, "failed to clear storage in SetupSubTest")
}

func (s *BlockStorageTestSuite) TearDownSuite() {
	s.cancellation()
}

func TestBlockStorageTestSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(BlockStorageTestSuite))
}

func (s *BlockStorageTestSuite) TestSetBlockBatchSequentially_GetConcurrently() {
	const blocksCount = 5
	batches := testaide.NewBatchesSequence(blocksCount)

	for _, batch := range batches {
		err := s.bs.SetBlockBatch(s.ctx, batch)
		s.Require().NoError(err)
	}

	waitGroup := sync.WaitGroup{}
	waitGroup.Add(blocksCount)

	for _, batch := range batches {
		go func() {
			mainBlockId := scTypes.IdFromBlock(batch.MainShardBlock)
			fromDb, err := s.bs.TryGetBlock(s.ctx, mainBlockId)
			s.NoError(err)
			s.NotNil(fromDb)
			s.Equal(batch.MainShardBlock.Number, fromDb.Number)
			s.Equal(batch.MainShardBlock.Hash, fromDb.Hash)
			waitGroup.Done()
		}()
	}

	waitGroup.Wait()
}

func (s *BlockStorageTestSuite) TestGetLastFetchedBlock() {
	// initially latestFetched should be empty
	latestFetched, err := s.bs.TryGetLatestFetched(s.ctx)
	s.Require().NoError(err)
	s.Require().Nil(latestFetched)

	batch := testaide.NewBlockBatch(3)
	err = s.bs.SetBlockBatch(s.ctx, batch)
	s.Require().NoError(err)

	// latestFetched is updated after the main shard block is saved
	latestFetched, err = s.bs.TryGetLatestFetched(s.ctx)
	s.Require().NoError(err)
	s.Require().NotNil(latestFetched)
	s.Require().Equal(batch.MainShardBlock.Number, latestFetched.Number)
	s.Require().Equal(batch.MainShardBlock.Hash, latestFetched.Hash)
}

func (s *BlockStorageTestSuite) TestSetBlockAsProved_DoesNotExist() {
	randomId := testaide.RandomBlockId()
	err := s.bs.SetBlockAsProved(s.ctx, randomId)
	s.Require().Errorf(err, "block with id=%s is not found", randomId.String())
}

func (s *BlockStorageTestSuite) TestSetBlockAsProved() {
	batch := testaide.NewBlockBatch(3)
	err := s.bs.SetBlockBatch(s.ctx, batch)
	s.Require().NoError(err)

	err = s.bs.SetBlockAsProved(s.ctx, scTypes.IdFromBlock(batch.MainShardBlock))
	s.Require().NoError(err)
}

func (s *BlockStorageTestSuite) TestSetBlockAsProposed_DoesNotExist() {
	randomId := testaide.RandomBlockId()
	err := s.bs.SetBlockAsProposed(s.ctx, randomId)
	s.Require().Errorf(err, "block with id=%s is not found", randomId.String())
}

func (s *BlockStorageTestSuite) TestSetBlockAsProposed_IsNotProved() {
	batch := testaide.NewBlockBatch(3)
	err := s.bs.SetBlockBatch(s.ctx, batch)
	s.Require().NoError(err)

	err = s.bs.SetBlockAsProposed(s.ctx, scTypes.IdFromBlock(batch.MainShardBlock))
	s.Require().Errorf(err, "block with hash=%s is not proved", batch.MainShardBlock.Hash.String())
}

func (s *BlockStorageTestSuite) TestSetBlockBatch_ParentHashMismatch() {
	prevBatch := testaide.NewBlockBatch(4)

	err := s.bs.SetBlockBatch(s.ctx, prevBatch)
	s.Require().NoError(err)

	newBatch := testaide.NewBlockBatch(4)
	newBatch.MainShardBlock.Number = prevBatch.MainShardBlock.Number + 1

	err = s.bs.SetBlockBatch(s.ctx, newBatch)
	s.Require().ErrorContains(err, "unable to update latest fetched block: block mismatch")
}

func (s *BlockStorageTestSuite) TestSetBlockBatch_ParentMismatch() {
	const childBlocksCount = 4

	testCases := []struct {
		name      string
		nextBatch func(prev *scTypes.BlockBatch) *scTypes.BlockBatch
	}{
		{
			name: "Main_Block_Hash_Mismatch",
			nextBatch: func(prev *scTypes.BlockBatch) *scTypes.BlockBatch {
				next := testaide.NewBlockBatch(childBlocksCount)
				next.MainShardBlock.ParentHash = testaide.RandomHash()
				next.MainShardBlock.Number = prev.MainShardBlock.Number + 1
				return next
			},
		},
		{
			name: "Main_Block_Number_Mismatch",
			nextBatch: func(prev *scTypes.BlockBatch) *scTypes.BlockBatch {
				next := testaide.NewBlockBatch(childBlocksCount)
				next.MainShardBlock.ParentHash = prev.MainShardBlock.Hash
				next.MainShardBlock.Number = testaide.RandomBlockNum()
				return next
			},
		},
	}

	for _, testCase := range testCases {
		s.Run(testCase.name, func() {
			prevBatch := testaide.NewBlockBatch(childBlocksCount)
			err := s.bs.SetBlockBatch(s.ctx, prevBatch)
			s.Require().NoError(err)

			nextBatch := testCase.nextBatch(prevBatch)
			err = s.bs.SetBlockBatch(s.ctx, nextBatch)
			s.Require().ErrorContains(err, "unable to update latest fetched block: block mismatch")
		})
	}
}

func (s *BlockStorageTestSuite) TestSetBlockAsProposed_WithExecutionShardBlocks() {
	const childBlocksCount = 3
	batch := testaide.NewBlockBatch(childBlocksCount)
	mainBlockId := scTypes.IdFromBlock(batch.MainShardBlock)

	err := s.bs.SetBlockBatch(s.ctx, batch)
	s.Require().NoError(err)

	nextBatch := testaide.NewBlockBatch(childBlocksCount)
	nextBatch.MainShardBlock.Number = batch.MainShardBlock.Number + 1
	nextBatch.MainShardBlock.ParentHash = batch.MainShardBlock.Hash
	err = s.bs.SetBlockBatch(s.ctx, nextBatch)
	s.Require().NoError(err)

	err = s.bs.SetBlockAsProved(s.ctx, mainBlockId)
	s.Require().NoError(err)

	err = s.bs.SetBlockAsProposed(s.ctx, mainBlockId)
	s.Require().NoError(err)

	allBlocks := make([]*jsonrpc.RPCBlock, 0)
	allBlocks = append(allBlocks, batch.AllBlocks()...)
	allBlocks = append(allBlocks, batch.AllBlocks()...)

	for _, block := range allBlocks {
		blockFromDb, err := s.bs.TryGetBlock(s.ctx, scTypes.IdFromBlock(block))
		s.Require().NoError(err)
		s.Require().Nil(blockFromDb)
	}
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

	batch := testaide.NewBlockBatch(3)
	err = s.bs.SetBlockBatch(s.ctx, batch)
	s.Require().NoError(err)

	data, err := s.bs.TryGetNextProposalData(s.ctx)
	s.Require().Nil(data)
	s.Require().NoError(err)
}

func (s *BlockStorageTestSuite) TestTryGetNextProposalData_Collect_Transactions() {
	err := s.bs.SetProvedStateRoot(s.ctx, testaide.RandomHash())
	s.Require().NoError(err, "failed to set initial state root")

	var expectedTxCount int

	const blocksCount = 3
	batch := testaide.NewBlockBatch(blocksCount)
	expectedTxCount += len(batch.MainShardBlock.Messages)

	for _, child := range batch.ChildBlocks {
		expectedTxCount += len(child.Messages)
	}

	err = s.bs.SetBlockBatch(s.ctx, batch)
	s.Require().NoError(err)

	err = s.bs.SetBlockAsProved(s.ctx, scTypes.IdFromBlock(batch.MainShardBlock))
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
	batches := testaide.NewBatchesSequence(blocksCount)

	for _, batch := range batches {
		s.Require().NoError(err)
		err = s.bs.SetBlockBatch(s.ctx, batch)
		s.Require().NoError(err, "failed to set block batch")
	}

	waitGroup := sync.WaitGroup{}
	waitGroup.Add(blocksCount + 1)

	// concurrently set blocks as proved
	for _, block := range batches {
		go func() {
			err := s.bs.SetBlockAsProved(s.ctx, scTypes.IdFromBlock(block.MainShardBlock))
			s.NoError(err, "failed to set block as proved")
			waitGroup.Done()
		}()
	}

	receiveTimeout := time.After(time.Second * 3)
	var receivedData []*scTypes.ProposalData
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
		batch := batches[idx]
		data := receivedData[idx]

		expectedTxCount := len(batch.MainShardBlock.Messages)
		for _, child := range batch.ChildBlocks {
			expectedTxCount += len(child.Messages)
		}

		s.Equal(batch.MainShardBlock.Hash, data.MainShardBlockHash, msg("MainShardBlockHash"))
		s.Len(data.Transactions, expectedTxCount, msg("Transactions count"))
		s.Equal(batch.MainShardBlock.ChildBlocksRootHash, data.NewProvedStateRoot, msg("NewProvedStateRoot"))

		if idx == 0 {
			s.Equal(initialStateRoot, data.OldProvedStateRoot, msg("OldProvedStateRoot"))
		} else {
			s.Equal(batches[idx-1].MainShardBlock.ChildBlocksRootHash, data.OldProvedStateRoot, msg("OldProvedStateRoot"))
		}
	}
}
