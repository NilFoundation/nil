package storage

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/metrics"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/testaide"
	scTypes "github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
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
	timer := common.NewTimer()
	metricsHandler, err := metrics.NewSyncCommitteeMetrics()
	s.Require().NoError(err)
	return NewBlockStorage(s.db, config, timer, metricsHandler, logging.NewLogger("block_storage_test"))
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

func (s *BlockStorageTestSuite) Test_SetBlockBatch_Capacity_Exceeded() {
	allowedBatchesCount := int(DefaultBlockStorageConfig().StoredBatchesLimit)
	batches := testaide.NewBatchesSequence(allowedBatchesCount)

	for _, batch := range batches {
		err := s.bs.SetBlockBatch(s.ctx, batch)
		s.Require().NoError(err)
	}

	lastStoredBatch := batches[len(batches)-1]

	nextBatch := testaide.NewBlockBatch(testaide.ShardsCount)
	nextBatch.MainShardBlock.Number = lastStoredBatch.MainShardBlock.Number + 1
	nextBatch.MainShardBlock.ParentHash = lastStoredBatch.MainShardBlock.Hash

	err := s.bs.SetBlockBatch(s.ctx, nextBatch)
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

	provedBatchId := batches[0].Id
	err = storage.SetBatchAsProved(s.ctx, provedBatchId)
	s.Require().NoError(err)
	err = storage.SetBatchAsProposed(s.ctx, provedBatchId)
	s.Require().NoError(err)

	err = storage.SetBlockBatch(s.ctx, batches[1])
	s.Require().NoError(err)
}

func (s *BlockStorageTestSuite) Test_LatestBatchId() {
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
	err := s.bs.SetBlockBatch(s.ctx, batch)
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

func (s *BlockStorageTestSuite) Test_SetBatchAsProposed_WithExecutionShardBlocks() {
	const childBlocksCount = 3
	batch := testaide.NewBlockBatch(childBlocksCount)

	err := s.bs.SetBlockBatch(s.ctx, batch)
	s.Require().NoError(err)

	nextBatch := testaide.NewBlockBatch(childBlocksCount)
	nextBatch.ParentId = &batch.Id
	nextBatch.MainShardBlock.Number = batch.MainShardBlock.Number + 1
	nextBatch.MainShardBlock.ParentHash = batch.MainShardBlock.Hash
	err = s.bs.SetBlockBatch(s.ctx, nextBatch)
	s.Require().NoError(err)

	err = s.bs.SetBatchAsProved(s.ctx, batch.Id)
	s.Require().NoError(err)

	err = s.bs.SetBatchAsProposed(s.ctx, batch.Id)
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

	var expectedTxCount int

	const blocksCount = 3
	batch := testaide.NewBlockBatch(blocksCount)
	expectedTxCount += len(batch.MainShardBlock.Transactions)

	for _, child := range batch.ChildBlocks {
		expectedTxCount += len(child.Transactions)
	}

	err = s.bs.SetBlockBatch(s.ctx, batch)
	s.Require().NoError(err)

	err = s.bs.SetBatchAsProved(s.ctx, batch.Id)
	s.Require().NoError(err)

	data, err := s.bs.TryGetNextProposalData(s.ctx)
	s.Require().NoError(err)
	s.Require().NotNil(data)
	s.Require().Len(data.Transactions, expectedTxCount)
}

func (s *BlockStorageTestSuite) Test_TryGetNextProposalData_Concurrently() {
	initialStateRoot := testaide.RandomHash()
	err := s.bs.SetProvedStateRoot(s.ctx, initialStateRoot)
	s.Require().NoError(err, "failed to set initial state root")

	const batchesCount = 10
	batches := testaide.NewBatchesSequence(batchesCount)

	for _, batch := range batches {
		s.Require().NoError(err)
		err = s.bs.SetBlockBatch(s.ctx, batch)
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
		for {
			if len(receivedData) == batchesCount {
				break
			}
			select {
			case <-s.ctx.Done():
				return s.ctx.Err()
			case <-receiveTimeout:
				return errors.New("proposal data receive timeout exceeded")
			default:
				data, err := s.bs.TryGetNextProposalData(s.ctx)
				if err != nil {
					return fmt.Errorf("failed to get next proposal data: %w", err)
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

	err = proofGroup.Wait()
	s.Require().NoError(err)

	txn := func(field string) string {
		return field + " is not equal to the expected value"
	}

	// check that data was received in correct order
	for idx := range batchesCount {
		batch := batches[idx]
		data := receivedData[idx]

		expectedTxCount := len(batch.MainShardBlock.Transactions)
		for _, child := range batch.ChildBlocks {
			expectedTxCount += len(child.Transactions)
		}

		s.Equal(batch.MainShardBlock.Hash, data.MainShardBlockHash, txn("MainShardBlockHash"))
		s.Len(data.Transactions, expectedTxCount, txn("Transactions count"))
		s.Equal(batch.MainShardBlock.Hash, data.NewProvedStateRoot, txn("NewProvedStateRoot"))

		if idx == 0 {
			s.Equal(initialStateRoot, data.OldProvedStateRoot, txn("OldProvedStateRoot"))
		} else {
			s.Equal(batches[idx-1].MainShardBlock.Hash, data.OldProvedStateRoot, txn("OldProvedStateRoot"))
		}
	}
}

const resetTestBatchesCount = 10

func (s *BlockStorageTestSuite) Test_ResetBatchesRange_Block_Does_Not_Exists() {
	batches := testaide.NewBatchesSequence(resetTestBatchesCount)

	for _, batch := range batches {
		err := s.bs.SetBlockBatch(s.ctx, batch)
		s.Require().NoError(err)
	}

	latestFetchedBeforeReset, err := s.bs.TryGetLatestFetched(s.ctx)
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

	latestFetchedAfterReset, err := s.bs.TryGetLatestFetched(s.ctx)
	s.Require().NoError(err)
	s.Require().Equal(latestFetchedBeforeReset, latestFetchedAfterReset)

	latestBatchIdAfterReset, err := s.bs.TryGetLatestBatchId(s.ctx)
	s.Require().NoError(err)
	s.Require().Equal(latestBatchIdBeforeReset, latestBatchIdAfterReset)
}

func (s *BlockStorageTestSuite) Test_ResetBatchesRange() {
	testCases := []struct {
		name          string
		passedHashIdx int
	}{
		{"First_Block_In_Chain", 0},
		{"Latest_Fetched_Only", resetTestBatchesCount - 1},
		{"Keep_Previous_Purge_Next", 5},
	}

	for _, testCase := range testCases {
		s.Run(testCase.name, func() {
			check.PanicIfNotf(
				testCase.passedHashIdx >= 0 && testCase.passedHashIdx < resetTestBatchesCount,
				"passedHashIdx should be in range [0, %d)", resetTestBatchesCount,
			)
			s.testResetBatchesRange(testCase.passedHashIdx)
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

	var expectedLatestFetched *scTypes.MainBlockRef
	if firstBatchToPurgeIdx == 0 {
		expectedLatestFetched = nil
	} else {
		expectedLatestFetched, _ = scTypes.NewBlockRef(batches[firstBatchToPurgeIdx-1].MainShardBlock)
	}

	actualLatestFetched, err := s.bs.TryGetLatestFetched(s.ctx)
	s.Require().NoError(err)
	s.Require().Equal(expectedLatestFetched, actualLatestFetched)

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

	latestFetched, err := s.bs.TryGetLatestFetched(s.ctx)
	s.Require().NoError(err)
	s.Require().Nil(latestFetched)

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

	latestFetched, err := storage.TryGetLatestFetched(s.ctx)
	s.Require().NoError(err)
	s.Require().Nil(latestFetched)

	for _, batch := range batches {
		fromStorage, err := s.bs.TryGetBlock(s.ctx, scTypes.IdFromBlock(batch.MainShardBlock))
		s.Require().NoError(err)
		s.Require().Nil(fromStorage)
	}
}

func (s *BlockStorageTestSuite) requireBatch(batch *scTypes.BlockBatch, shouldBePurged bool) {
	s.T().Helper()
	for _, block := range batch.AllBlocks() {
		fromStorage, err := s.bs.TryGetBlock(s.ctx, scTypes.IdFromBlock(block))
		s.Require().NoError(err)
		if shouldBePurged {
			s.Nil(fromStorage)
		} else {
			s.NotNil(fromStorage)
		}
	}
}
