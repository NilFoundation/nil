package storage

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/metrics"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/testaide"
	scTypes "github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/suite"
	"golang.org/x/sync/errgroup"
)

type BlockStorageTestSuite struct {
	suite.Suite

	db           db.DB
	ctx          context.Context
	cancellation context.CancelFunc
	bs           *BlockStorage
}

func (s *BlockStorageTestSuite) SetupSuite() {
	s.ctx, s.cancellation = context.WithCancel(context.Background())

	var err error
	s.db, err = db.NewBadgerDbInMemory()
	s.Require().NoError(err)
	config := DefaultBlockStorageConfig()
	s.bs = s.newTestBlockStorage(config)
}

func (s *BlockStorageTestSuite) newTestBlockStorage(config BlockStorageConfig) *BlockStorage {
	s.T().Helper()
	clock := clockwork.NewRealClock()
	metricsHandler, err := metrics.NewSyncCommitteeMetrics()
	s.Require().NoError(err)
	return NewBlockStorage(s.db, config, clock, metricsHandler, logging.NewLogger("block_storage_test"))
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
			mainBlock := batch.LatestMainBlock()
			mainBlockId := scTypes.IdFromBlock(mainBlock)
			fromDb, err := s.bs.TryGetBlock(s.ctx, mainBlockId)
			s.NoError(err)
			s.NotNil(fromDb)
			s.Equal(mainBlock.Number, fromDb.Number)
			s.Equal(mainBlock.Hash, fromDb.Hash)
			waitGroup.Done()
		}()
	}

	waitGroup.Wait()
}

func (s *BlockStorageTestSuite) Test_SetBlockBatch_Capacity_Exceeded() {
	allowedBatchesCount := int(DefaultBlockStorageConfig().StoredBatchesLimit)
	batches := testaide.NewBatchesSequence(allowedBatchesCount + 1)

	for _, batch := range batches[:allowedBatchesCount] {
		err := s.bs.SetBlockBatch(s.ctx, batch)
		s.Require().NoError(err)
	}

	err := s.bs.SetBlockBatch(s.ctx, batches[len(batches)-1])
	s.Require().ErrorIs(err, ErrCapacityLimitReached)
}

func (s *BlockStorageTestSuite) Test_SetBlockBatch_Free_Capacity_On_SetBatchAsProposed() {
	batches := testaide.NewBatchesSequence(2)

	const capacityLimit = 1
	config := NewBlockStorageConfig(capacityLimit)
	storage := s.newTestBlockStorage(config)

	err := storage.SetBlockBatch(s.ctx, batches[0])
	s.Require().NoError(err)

	err = storage.SetBlockBatch(s.ctx, batches[1])
	s.Require().ErrorIs(err, ErrCapacityLimitReached)

	provedBatch := batches[0]
	provedBatchId := provedBatch.Id
	err = storage.SetBatchAsProved(s.ctx, provedBatchId)
	s.Require().NoError(err)

	err = storage.SetProvedStateRoot(s.ctx, provedBatch.FirstMainBlock().ParentHash)
	s.Require().NoError(err)

	err = storage.SetBatchAsProposed(s.ctx, provedBatchId)
	s.Require().NoError(err)

	err = storage.SetBlockBatch(s.ctx, batches[1])
	s.Require().NoError(err)
}

func (s *BlockStorageTestSuite) Test_TryGetLatestBatchId() {
	const batchesCount = 5
	batches := testaide.NewBatchesSequence(batchesCount)

	latestBatchId, err := s.bs.TryGetLatestBatchId(s.ctx)
	s.Require().NoError(err)
	s.Require().Nil(latestBatchId)

	for _, batch := range batches {
		err := s.bs.SetBlockBatch(s.ctx, batch)
		s.Require().NoError(err)

		latestBatchId, err := s.bs.TryGetLatestBatchId(s.ctx)
		s.Require().NoError(err)
		s.Equal(&batch.Id, latestBatchId)
	}
}

func (s *BlockStorageTestSuite) Test_BatchExists_True() {
	batch := testaide.NewBlockBatch(3)
	err := s.bs.SetBlockBatch(s.ctx, batch)
	s.Require().NoError(err)

	exists, err := s.bs.BatchExists(s.ctx, batch.Id)
	s.Require().NoError(err)
	s.Require().True(exists)
}

func (s *BlockStorageTestSuite) Test_BatchExists_False() {
	batch := testaide.NewBlockBatch(3)
	err := s.bs.SetBlockBatch(s.ctx, batch)
	s.Require().NoError(err)

	exists, err := s.bs.BatchExists(s.ctx, scTypes.NewBatchId())
	s.Require().NoError(err)
	s.Require().False(exists)
}

func (s *BlockStorageTestSuite) Test_LatestBatchId_Mismatch() {
	const batchesCount = 2
	batches := testaide.NewBatchesSequence(batchesCount)
	invalidParentId := scTypes.NewBatchId()
	batches[1].ParentId = &invalidParentId

	err := s.bs.SetBlockBatch(s.ctx, batches[0])
	s.Require().NoError(err)

	err = s.bs.SetBlockBatch(s.ctx, batches[1])
	s.Require().ErrorIs(err, scTypes.ErrBatchMismatch)
}

func (s *BlockStorageTestSuite) Test_GetLatestFetched() {
	// initially latestFetched should be empty
	latestFetched, err := s.bs.GetLatestFetched(s.ctx)
	s.Require().NoError(err)
	s.Require().Empty(latestFetched)

	batch := testaide.NewBlockBatch(3)
	err = s.bs.SetBlockBatch(s.ctx, batch)
	s.Require().NoError(err)

	// latestFetched is updated after batch is saved
	latestFetched, err = s.bs.GetLatestFetched(s.ctx)
	s.Require().NoError(err)
	s.Require().NotEmpty(latestFetched)

	for _, ref := range batch.LatestRefs() {
		latestFromDb, ok := latestFetched[ref.ShardId]
		s.Require().True(ok)
		s.Equal(ref.Number, latestFromDb.Number)
		s.Equal(ref.Hash, latestFromDb.Hash)
	}
}

func (s *BlockStorageTestSuite) Test_SetBatchAsProved_Batch_Does_Not_Exist() {
	randomId := scTypes.NewBatchId()
	err := s.bs.SetBatchAsProved(s.ctx, randomId)
	s.Require().ErrorIs(err, scTypes.ErrBatchNotFound)
}

func (s *BlockStorageTestSuite) Test_SetBatchAsProved() {
	batch := testaide.NewBlockBatch(3)
	err := s.bs.SetBlockBatch(s.ctx, batch)
	s.Require().NoError(err)

	err = s.bs.SetBatchAsProved(s.ctx, batch.Id)
	s.Require().NoError(err)
}

func (s *BlockStorageTestSuite) Test_SetBatchAsProved_Multiple_Times() {
	batch := testaide.NewBlockBatch(3)

	err := s.bs.SetProvedStateRoot(s.ctx, batch.FirstMainBlock().ParentHash)
	s.Require().NoError(err)

	err = s.bs.SetBlockBatch(s.ctx, batch)
	s.Require().NoError(err)

	for range 3 {
		err := s.bs.SetBatchAsProved(s.ctx, batch.Id)
		s.Require().NoError(err)
	}

	err = s.bs.SetBatchAsProposed(s.ctx, batch.Id)
	s.Require().NoError(err)
}

func (s *BlockStorageTestSuite) Test_SetBatchAsProposed_Batch_Does_Not_Exist() {
	randomId := scTypes.NewBatchId()
	err := s.bs.SetBatchAsProposed(s.ctx, randomId)
	s.Require().ErrorIs(err, scTypes.ErrBatchNotFound)
}

func (s *BlockStorageTestSuite) Test_SetBatchAsProposed_Batch_Is_Not_Proved() {
	batch := testaide.NewBlockBatch(3)
	err := s.bs.SetBlockBatch(s.ctx, batch)
	s.Require().NoError(err)

	err = s.bs.SetBatchAsProposed(s.ctx, batch.Id)
	s.Require().ErrorIs(err, scTypes.ErrBatchNotProved)
}

func (s *BlockStorageTestSuite) Test_SetBlockBatch_ParentHashMismatch() {
	prevBatch := testaide.NewBlockBatch(4)

	err := s.bs.SetBlockBatch(s.ctx, prevBatch)
	s.Require().NoError(err)

	newBatch := testaide.NewBlockBatch(4)
	newBatch.FirstMainBlock().Number = prevBatch.LatestMainBlock().Number + 1

	err = s.bs.SetBlockBatch(s.ctx, newBatch)
	s.Require().ErrorIs(err, scTypes.ErrBatchMismatch)
	s.Require().ErrorContains(err, "does not match current ref")
}

func (s *BlockStorageTestSuite) TestSetBlockBatch_ParentMismatch() {
	const childBlocksCount = 4

	testCases := []struct {
		name          string
		batchModifier func(batch *scTypes.BlockBatch) *scTypes.BlockBatch
	}{
		{
			name: "Main_Block_Hash_Mismatch",
			batchModifier: func(prev *scTypes.BlockBatch) *scTypes.BlockBatch {
				next := testaide.NewBlockBatch(childBlocksCount)
				next.FirstMainBlock().ParentHash = testaide.RandomHash()
				next.FirstMainBlock().Number = prev.LatestMainBlock().Number + 1
				return next
			},
		},
		{
			name: "Main_Block_Number_Mismatch",
			batchModifier: func(prev *scTypes.BlockBatch) *scTypes.BlockBatch {
				next := testaide.NewBlockBatch(childBlocksCount)
				next.FirstMainBlock().ParentHash = prev.LatestMainBlock().Hash
				next.FirstMainBlock().Number = testaide.RandomBlockNum()
				return next
			},
		},
	}

	for _, testCase := range testCases {
		s.Run(testCase.name, func() {
			batches := testaide.NewBatchesSequence(2)
			err := s.bs.SetBlockBatch(s.ctx, batches[0])
			s.Require().NoError(err)

			nextBatch := testCase.batchModifier(batches[1])
			err = s.bs.SetBlockBatch(s.ctx, nextBatch)
			s.Require().ErrorIs(err, scTypes.ErrBatchMismatch)
			s.Require().ErrorContains(err, "does not match current ref")
		})
	}
}

func (s *BlockStorageTestSuite) Test_SetBatchAsProposed_WithExecutionShardBlocks() {
	batches := testaide.NewBatchesSequence(2)

	err := s.bs.SetBlockBatch(s.ctx, batches[0])
	s.Require().NoError(err)

	err = s.bs.SetBlockBatch(s.ctx, batches[1])
	s.Require().NoError(err)

	err = s.bs.SetBatchAsProved(s.ctx, batches[0].Id)
	s.Require().NoError(err)

	err = s.bs.SetProvedStateRoot(s.ctx, batches[0].LatestMainBlock().ParentHash)
	s.Require().NoError(err)

	err = s.bs.SetBatchAsProposed(s.ctx, batches[0].Id)
	s.Require().NoError(err)

	for block := range batches[0].BlocksIter() {
		blockFromDb, err := s.bs.TryGetBlock(s.ctx, scTypes.IdFromBlock(block))
		s.Require().NoError(err)
		s.Require().Nil(blockFromDb)
	}
}

func (s *BlockStorageTestSuite) Test_TryGetNextProposalData_NotInitializedStateRoot() {
	data, err := s.bs.TryGetNextProposalData(s.ctx)
	s.Require().Nil(data)
	s.Require().Error(err, "proved state root was not initialized")
}

func (s *BlockStorageTestSuite) Test_TryGetNextProposalData_BlockParentHashNotSet() {
	err := s.bs.SetProvedStateRoot(s.ctx, testaide.RandomHash())
	s.Require().NoError(err)

	data, err := s.bs.TryGetNextProposalData(s.ctx)
	s.Require().Nil(data)
	s.Require().NoError(err)
}

func (s *BlockStorageTestSuite) Test_TryGetNextProposalData_NoProvedMainShardBlockFound() {
	err := s.bs.SetProvedStateRoot(s.ctx, testaide.RandomHash())
	s.Require().NoError(err)

	batch := testaide.NewBlockBatch(3)
	err = s.bs.SetBlockBatch(s.ctx, batch)
	s.Require().NoError(err)

	data, err := s.bs.TryGetNextProposalData(s.ctx)
	s.Require().Nil(data)
	s.Require().NoError(err)
}

func (s *BlockStorageTestSuite) Test_TryGetNextProposalData_Collect_Transactions() {
	err := s.bs.SetProvedStateRoot(s.ctx, testaide.RandomHash())
	s.Require().NoError(err, "failed to set initial state root")

	const blocksCount = 3
	batch := testaide.NewBlockBatch(blocksCount)
	var expectedTxCount int
	for block := range batch.BlocksIter() {
		expectedTxCount += len(block.Transactions)
	}

	err = s.bs.SetBlockBatch(s.ctx, batch)
	s.Require().NoError(err)

	err = s.bs.SetBatchAsProved(s.ctx, batch.Id)
	s.Require().NoError(err)

	err = s.bs.SetProvedStateRoot(s.ctx, batch.FirstMainBlock().ParentHash)
	s.Require().NoError(err)

	data, err := s.bs.TryGetNextProposalData(s.ctx)
	s.Require().NoError(err)
	s.Require().NotNil(data)
}

func (s *BlockStorageTestSuite) Test_TryGetNextProposalData_Concurrently() {
	const batchesCount = 10
	batches := testaide.NewBatchesSequence(batchesCount)

	for _, batch := range batches {
		err := s.bs.SetBlockBatch(s.ctx, batch)
		s.Require().NoError(err, "failed to set block batch")
	}

	var proofGroup errgroup.Group
	for _, batch := range batches {
		proofGroup.Go(func() error {
			return s.bs.SetBatchAsProved(s.ctx, batch.Id)
		})
	}

	receiveTimeout := time.After(time.Second * 3)
	var receivedData []*scTypes.ProposalData
	proofGroup.Go(func() error {
		// poll all blocks data from the storage
		for len(receivedData) != batchesCount {
			select {
			case <-s.ctx.Done():
				return s.ctx.Err()
			case <-receiveTimeout:
				return errors.New("proposal data receive timeout exceeded")
			default:
				data, err := s.bs.TryGetNextProposalData(s.ctx)
				if err != nil {
					s.Require().ErrorIs(err, ErrStateRootNotInitialized)
					continue
				}
				if data == nil {
					continue
				}

				receivedData = append(receivedData, data)
				err = s.bs.SetBatchAsProposed(s.ctx, data.BatchId)
				if err != nil {
					return fmt.Errorf("failed to set batch %s as proposed: %w", data.BatchId, err)
				}
			}
		}
		return nil
	})

	proofGroup.Go(func() error {
		return s.bs.SetProvedStateRoot(s.ctx, batches[0].FirstMainBlock().ParentHash)
	})

	err := proofGroup.Wait()
	s.Require().NoError(err)

	txn := func(field string) string {
		return field + " is not equal to the expected value"
	}

	// check that data was received in correct order
	for idx := range batchesCount {
		batch := batches[idx]
		data := receivedData[idx]

		mainRef := batch.LatestRefs().TryGetMain()
		s.Equal(mainRef.Hash, data.NewProvedStateRoot, txn("NewProvedStateRoot"))

		parentMainRef := batch.ParentRefs()[types.MainShardId]
		s.Require().NotNil(parentMainRef)
		s.Equal(parentMainRef.Hash, data.OldProvedStateRoot, txn("OldProvedStateRoot"))
	}
}

const resetTestBatchesCount = 10

func (s *BlockStorageTestSuite) Test_ResetBatchesRange_Block_Does_Not_Exists() {
	batches := testaide.NewBatchesSequence(resetTestBatchesCount)

	for _, batch := range batches {
		err := s.bs.SetBlockBatch(s.ctx, batch)
		s.Require().NoError(err)
	}

	latestFetchedBeforeReset, err := s.bs.GetLatestFetched(s.ctx)
	s.Require().NoError(err)
	latestBatchIdBeforeReset, err := s.bs.TryGetLatestBatchId(s.ctx)
	s.Require().NoError(err)

	nonExistentBatchId := scTypes.NewBatchId()
	purgedBatches, err := s.bs.ResetBatchesRange(s.ctx, nonExistentBatchId)
	s.Require().ErrorIs(err, scTypes.ErrBatchNotFound)
	s.Require().Empty(purgedBatches)

	for _, batch := range batches {
		s.requireBatch(batch, false)
	}

	latestFetchedAfterReset, err := s.bs.GetLatestFetched(s.ctx)
	s.Require().NoError(err)
	s.Require().Equal(latestFetchedBeforeReset, latestFetchedAfterReset)

	latestBatchIdAfterReset, err := s.bs.TryGetLatestBatchId(s.ctx)
	s.Require().NoError(err)
	s.Require().Equal(latestBatchIdBeforeReset, latestBatchIdAfterReset)
}

func (s *BlockStorageTestSuite) Test_ResetBatchesRange() {
	testCases := []struct {
		name                 string
		firstBatchToPurgeIdx int
	}{
		{"First_Block_In_Chain", 0},
		{"Latest_Fetched_Only", resetTestBatchesCount - 1},
		{"Keep_Previous_Purge_Next", 5},
	}

	for _, testCase := range testCases {
		s.Run(testCase.name, func() {
			check.PanicIfNotf(
				testCase.firstBatchToPurgeIdx >= 0 && testCase.firstBatchToPurgeIdx < resetTestBatchesCount,
				"firstBatchToPurgeIdx should be in range [0, %d)", resetTestBatchesCount,
			)
			s.testResetBatchesRange(testCase.firstBatchToPurgeIdx)
		})
	}
}

func (s *BlockStorageTestSuite) testResetBatchesRange(firstBatchToPurgeIdx int) {
	s.T().Helper()

	batches := testaide.NewBatchesSequence(resetTestBatchesCount)

	for _, batch := range batches {
		err := s.bs.SetBlockBatch(s.ctx, batch)
		s.Require().NoError(err)
	}

	firstBatchIdToPurge := batches[firstBatchToPurgeIdx].Id
	purgedBatches, err := s.bs.ResetBatchesRange(s.ctx, firstBatchIdToPurge)
	s.Require().NoError(err)
	s.Require().Len(purgedBatches, len(batches)-firstBatchToPurgeIdx)
	for i, batch := range batches[firstBatchToPurgeIdx:] {
		s.Require().Equal(batch.Id, purgedBatches[i])
	}

	for i, batch := range batches {
		shouldBePurged := i >= firstBatchToPurgeIdx
		s.requireBatch(batch, shouldBePurged)
	}

	actualLatestFetched, err := s.bs.GetLatestFetched(s.ctx)
	s.Require().NoError(err)

	if firstBatchToPurgeIdx == 0 {
		s.Require().Empty(actualLatestFetched)
	} else {
		expectedNewLatest := batches[firstBatchToPurgeIdx-1].LatestRefs()
		s.Require().Equal(expectedNewLatest, actualLatestFetched)
	}

	actualLatestBatchId, err := s.bs.TryGetLatestBatchId(s.ctx)
	s.Require().NoError(err)
	s.Require().Equal(batches[firstBatchToPurgeIdx].ParentId, actualLatestBatchId)
}

func (s *BlockStorageTestSuite) Test_ResetBatchesNotProved() {
	batches := testaide.NewBatchesSequence(resetTestBatchesCount)

	for _, batch := range batches {
		err := s.bs.SetBlockBatch(s.ctx, batch)
		s.Require().NoError(err)
	}

	const provedBatchesCount = 3
	provedBatches := batches[:provedBatchesCount]
	for _, batch := range provedBatches {
		err := s.bs.SetBatchAsProved(s.ctx, batch.Id)
		s.Require().NoError(err)
	}

	err := s.bs.ResetBatchesNotProved(s.ctx)
	s.Require().NoError(err)

	latestFetched, err := s.bs.GetLatestFetched(s.ctx)
	s.Require().NoError(err)
	s.Require().Empty(latestFetched)

	for _, provedBatch := range provedBatches {
		s.requireBatch(provedBatch, false)
	}

	for _, notProvenBatch := range batches[provedBatchesCount:] {
		s.requireBatch(notProvenBatch, true)
	}
}

func (s *BlockStorageTestSuite) Test_ResetBatchesNotProved_1K_Batches_To_Purge() {
	capacityLimit := uint32(1_000)
	config := NewBlockStorageConfig(capacityLimit)
	storage := s.newTestBlockStorage(config)

	batchesCount := int(capacityLimit)
	batches := testaide.NewBatchesSequence(batchesCount)

	for _, batch := range batches {
		err := storage.SetBlockBatch(s.ctx, batch)
		s.Require().NoError(err)
	}

	err := storage.ResetBatchesNotProved(s.ctx)
	s.Require().NoError(err)

	latestFetched, err := storage.GetLatestFetched(s.ctx)
	s.Require().NoError(err)
	s.Require().Empty(latestFetched)

	for _, batch := range batches {
		fromStorage, err := s.bs.TryGetBlock(s.ctx, scTypes.IdFromBlock(batch.LatestMainBlock()))
		s.Require().NoError(err)
		s.Require().Nil(fromStorage)
	}
}

func (s *BlockStorageTestSuite) requireBatch(batch *scTypes.BlockBatch, shouldBePurged bool) {
	s.T().Helper()
	for block := range batch.BlocksIter() {
		fromStorage, err := s.bs.TryGetBlock(s.ctx, scTypes.IdFromBlock(block))
		s.Require().NoError(err)
		if shouldBePurged {
			s.Nil(fromStorage)
		} else {
			s.NotNil(fromStorage)
		}
	}
}
