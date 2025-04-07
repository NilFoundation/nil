package core

import (
	"testing"

	coreTypes "github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/testaide"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	"github.com/stretchr/testify/suite"
)

type BlockBatchTestSuite struct {
	suite.Suite
}

func TestBlockBatchTestSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(BlockBatchTestSuite))
}

func (s *BlockBatchTestSuite) Test_Valid_Batch_Single_ChainSegments() {
	blocks := testaide.NewChainSegments(testaide.ShardsCount)
	batch, err := types.NewBlockBatch(nil).WithAddedBlocks(blocks)
	s.Require().NoError(err)
	s.Require().NotNil(batch)
	s.Require().Equal(blocks, batch.Blocks)
}

func (s *BlockBatchTestSuite) Test_Valid_Batch_Multiple_ChainSegments() {
	segments := testaide.NewSegmentsSequence(2)

	batch, err := types.NewBlockBatch(nil).WithAddedBlocks(segments[0])
	s.Require().NoError(err)
	s.Require().NotNil(batch)

	updatedBatch, err := batch.WithAddedBlocks(segments[1])
	s.Require().NoError(err)
	s.Require().NotNil(updatedBatch)

	expectedSegments, err := segments[0].Concat(segments[1])
	s.Require().NoError(err)
	s.Require().Equal(expectedSegments, updatedBatch.Blocks)
}

func (s *BlockBatchTestSuite) Test_Invalid_Sequencing() {
	segments := testaide.NewChainSegments(testaide.ShardsCount)

	batch, err := types.NewBlockBatch(nil).WithAddedBlocks(segments)
	s.Require().NoError(err)
	s.Require().NotNil(batch)

	// nextSegments has no connection with the first segments
	nextSegments := testaide.NewChainSegments(testaide.ShardsCount)

	updatedBatch, err := batch.WithAddedBlocks(nextSegments)
	s.Require().ErrorIs(err, types.ErrBlockMismatch)
	s.Require().Nil(updatedBatch)
}

func (s *BlockBatchTestSuite) Test_NewChainSegments_Corrupted_Order() {
	mainBlock := testaide.NewMainShardBlock()
	mainBlock.ChildBlocks = nil

	shardId := coreTypes.ShardId(1)

	firstBlock := testaide.NewExecutionShardBlock()
	firstBlock.ShardId = shardId

	secondBlock := testaide.NewExecutionShardBlock()
	secondBlock.ShardId = shardId
	secondBlock.Number = firstBlock.Number - 10
	secondBlock.ParentHash = firstBlock.Hash

	mainBlock.ChildBlocks = append(mainBlock.ChildBlocks, firstBlock.Hash, secondBlock.Hash)

	blocks := map[coreTypes.ShardId][]*types.Block{
		coreTypes.MainShardId: {mainBlock},
		shardId:               {firstBlock, secondBlock},
	}

	segments, err := types.NewChainSegments(blocks)
	s.Require().ErrorContains(err, "failed to create chain segment for shard "+shardId.String())
	s.Require().Nil(segments)
}

func (s *BlockBatchTestSuite) Test_CreateProofTask() {
	const childBLockCount = 4
	batch := testaide.NewBlockBatch(childBLockCount)

	taskEntry, err := batch.CreateProofTask(testaide.Now)
	s.Require().NoError(err)

	task := taskEntry.Task
	s.Require().Equal(types.WaitingForExecutor, taskEntry.Status)
	s.Require().Equal(types.ProofBatch, task.TaskType)
	s.Require().Equal(batch.Id, task.BatchId)
	s.Require().Nil(task.ParentTaskId)

	blockIds := batch.BlockIds()
	s.Require().Equal(blockIds, task.BlockIds)
}
