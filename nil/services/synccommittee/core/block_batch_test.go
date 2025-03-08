package core

import (
	"testing"

	"github.com/NilFoundation/nil/nil/common"
	coreTypes "github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
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

func (s *BlockBatchTestSuite) TestNewBlockBatch() {
	validBatch := testaide.NewBlockBatch(testaide.ShardsCount)

	notReadyBatch := testaide.NewBlockBatch(testaide.ShardsCount)
	notReadyBatch.MainShardBlock.ChildBlocks[0] = common.EmptyHash
	notReadyBatch.ChildBlocks[0] = nil

	nilChildBatch := testaide.NewBlockBatch(testaide.ShardsCount)
	nilChildBatch.ChildBlocks[1] = nil

	redundantChildBatch := testaide.NewBlockBatch(testaide.ShardsCount)
	redundantChildBatch.ChildBlocks = append(redundantChildBatch.ChildBlocks, testaide.NewExecutionShardBlock())

	hashMismatchBatch := testaide.NewBlockBatch(testaide.ShardsCount)
	hashMismatchBatch.ChildBlocks[2].Hash = testaide.RandomHash()

	testCases := []struct {
		name           string
		mainShardBlock *jsonrpc.RPCBlock
		childBlocks    []*jsonrpc.RPCBlock
		errPredicate   func(error)
	}{
		{
			name:           "Valid_Batch_No_Error",
			mainShardBlock: validBatch.MainShardBlock,
			childBlocks:    validBatch.ChildBlocks,
			errPredicate:   func(err error) { s.Require().NoError(err) },
		},
		{
			name:           "Nil_Main_Shard_Block",
			mainShardBlock: nil,
			childBlocks:    []*jsonrpc.RPCBlock{},
			errPredicate:   func(err error) { s.Require().ErrorContains(err, "mainShardBlock") },
		},
		{
			name:           "Valid_Main_Shard_Block_Nil_Child_Blocks",
			mainShardBlock: testaide.NewMainShardBlock(),
			childBlocks:    nil,
			errPredicate:   func(err error) { s.Require().ErrorContains(err, "childBlocks") },
		},
		{
			name:           "Not_Ready_Batch_Child_Hash_Is_Empty",
			mainShardBlock: notReadyBatch.MainShardBlock,
			childBlocks:    notReadyBatch.ChildBlocks,
			errPredicate:   func(err error) { s.Require().ErrorIs(err, types.ErrBatchNotReady) },
		},
		{
			name:           "Valid_Main_Shard_Block_Nil_Child_Block",
			mainShardBlock: nilChildBatch.MainShardBlock,
			childBlocks:    nilChildBatch.ChildBlocks,
			errPredicate: func(err error) {
				s.Require().NotErrorIs(err, types.ErrBatchNotReady)
				s.Require().ErrorContains(err, "childBlocks[1] cannot be nil")
			},
		},
		{
			name:           "Block_Is_Not_From_The_Main_Shard",
			mainShardBlock: testaide.NewExecutionShardBlock(),
			childBlocks:    []*jsonrpc.RPCBlock{},
			errPredicate:   func(err error) { s.Require().ErrorContains(err, "mainShardBlock is not from the main shard") },
		},
		{
			name:           "Redundant_Child_Block",
			mainShardBlock: redundantChildBatch.MainShardBlock,
			childBlocks:    redundantChildBatch.ChildBlocks,
			errPredicate:   func(err error) { s.Require().ErrorContains(err, "have different length") },
		},
		{
			name:           "Child_Hash_Mismatch",
			mainShardBlock: hashMismatchBatch.MainShardBlock,
			childBlocks:    hashMismatchBatch.ChildBlocks,
			errPredicate: func(err error) {
				s.Require().ErrorContains(err, "childBlocks[2].Hash != mainShardBlock.ChildBlocks[2]")
			},
		},
	}

	for _, testCase := range testCases {
		s.Run(testCase.name, func() {
			parentBatchId := types.NewBatchId()
			batch, err := types.NewBlockBatch(&parentBatchId, testCase.mainShardBlock, testCase.childBlocks)
			testCase.errPredicate(err)

			if err != nil {
				s.Require().Nil(batch)
				return
			}

			s.Require().NotNil(batch)
			s.Require().Equal(testCase.mainShardBlock, batch.MainShardBlock)
			s.Require().Equal(testCase.childBlocks, batch.ChildBlocks)
		})
	}
}

func (s *BlockBatchTestSuite) TestCreateProofTasks() {
	const childBLockCount = 4
	batch := testaide.NewBlockBatch(childBLockCount)

	taskEntries, err := batch.CreateProofTasks(testaide.Now)
	s.Require().NoError(err)

	s.Require().Len(taskEntries, childBLockCount+1)

	shardTasks := make(map[coreTypes.ShardId]types.Task)
	for _, entry := range taskEntries {
		shardTasks[entry.Task.ShardId] = entry.Task
	}

	mainShardTask, ok := shardTasks[coreTypes.MainShardId]
	s.Require().True(ok)

	s.Require().Equal(types.AggregateProofs, mainShardTask.TaskType)
	s.Require().Equal(batch.Id, mainShardTask.BatchId)
	s.Require().Equal(batch.MainShardBlock.Hash, mainShardTask.BlockHash)
	s.Require().Equal(batch.MainShardBlock.Number, mainShardTask.BlockNum)
	s.Require().Nil(mainShardTask.ParentTaskId)

	for _, childBlock := range batch.ChildBlocks {
		childTask, ok := shardTasks[childBlock.ShardId]
		s.Require().True(ok)

		s.Require().Equal(types.ProofBlock, childTask.TaskType)
		s.Require().Equal(batch.Id, childTask.BatchId)
		s.Require().Equal(childBlock.Hash, childTask.BlockHash)
		s.Require().Equal(childBlock.Number, childTask.BlockNum)
		s.Require().Equal(mainShardTask.Id, *childTask.ParentTaskId)
	}
}
