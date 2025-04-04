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

func (s *BlockBatchTestSuite) Test_Valid_Batch_Single_Subgraph() {
	subgraph := testaide.NewSubgraph(testaide.ShardsCount)
	batch, err := types.NewBlockBatch(nil).WithAddedSubgraph(subgraph)
	s.Require().NoError(err)
	s.Require().NotNil(batch)
	s.Require().Len(batch.Subgraphs, 1)
	s.Require().Equal(subgraph, batch.Subgraphs[0])
}

func (s *BlockBatchTestSuite) Test_Valid_Batch_Multiple_Subgraphs() {
	subgraphs := testaide.NewSubgraphSequence(2)

	batch, err := types.NewBlockBatch(nil).WithAddedSubgraph(subgraphs[0])
	s.Require().NoError(err)
	s.Require().NotNil(batch)

	updatedBatch, err := batch.WithAddedSubgraph(subgraphs[1])
	s.Require().NoError(err)
	s.Require().NotNil(updatedBatch)
	s.Require().Equal(subgraphs, updatedBatch.Subgraphs)
}

func (s *BlockBatchTestSuite) Test_Invalid_Sequencing() {
	subgraph := testaide.NewSubgraph(testaide.ShardsCount)

	batch, err := types.NewBlockBatch(nil).WithAddedSubgraph(subgraph)
	s.Require().NoError(err)
	s.Require().NotNil(batch)

	// nextSubgraph has no connection with the first subgraph
	nextSubgraph := testaide.NewSubgraph(testaide.ShardsCount)

	updatedBatch, err := batch.WithAddedSubgraph(nextSubgraph)
	s.Require().ErrorIs(err, types.ErrBlockMismatch)
	s.Require().Nil(updatedBatch)
}

func (s *BlockBatchTestSuite) Test_NewSubgraph_Corrupted_Order() {
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

	children := map[coreTypes.ShardId]types.ShardChainSegment{
		shardId: []*types.Block{firstBlock, secondBlock},
	}

	subgraph, err := types.NewSubgraph(mainBlock, children)
	s.Require().ErrorContains(err, "validation failed for shard "+shardId.String())
	s.Require().Nil(subgraph)
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
