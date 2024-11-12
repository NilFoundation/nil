package core

import (
	"testing"

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
	validBatch := testaide.GenerateBlockBatch(4)

	nilChildBatch := testaide.GenerateBlockBatch(4)
	nilChildBatch.ChildBlocks[1] = nil

	redundantChildBatch := testaide.GenerateBlockBatch(4)
	redundantChildBatch.ChildBlocks = append(redundantChildBatch.ChildBlocks, testaide.GenerateExecutionShardBlock())

	hashMismatchBatch := testaide.GenerateBlockBatch(4)
	hashMismatchBatch.ChildBlocks[2].Hash = testaide.RandomHash()

	testCases := []struct {
		name           string
		mainShardBlock *jsonrpc.RPCBlock
		childBlocks    []*jsonrpc.RPCBlock
		errPredicate   func(error)
	}{
		{
			name:           "valid batch, expect no error",
			mainShardBlock: validBatch.MainShardBlock,
			childBlocks:    validBatch.ChildBlocks,
			errPredicate:   func(err error) { s.Require().NoError(err) },
		},
		{
			name:           "nil mainShardBlock",
			mainShardBlock: nil,
			childBlocks:    []*jsonrpc.RPCBlock{},
			errPredicate:   func(err error) { s.Require().ErrorContains(err, "mainShardBlock") },
		},
		{
			name:           "valid mainShardBlock, nil childBlocks",
			mainShardBlock: testaide.GenerateMainShardBlock(),
			childBlocks:    nil,
			errPredicate:   func(err error) { s.Require().ErrorContains(err, "childBlocks") },
		},
		{
			name:           "valid mainShardBlock, nil childBlocks",
			mainShardBlock: nilChildBatch.MainShardBlock,
			childBlocks:    nilChildBatch.ChildBlocks,
			errPredicate:   func(err error) { s.Require().ErrorContains(err, "childBlocks[1] cannot be nil") },
		},
		{
			name:           "mainShardBlock is not from the main shard",
			mainShardBlock: testaide.GenerateExecutionShardBlock(),
			childBlocks:    []*jsonrpc.RPCBlock{},
			errPredicate:   func(err error) { s.Require().ErrorContains(err, "mainShardBlock is not from the main shard") },
		},
		{
			name:           "redundant child block",
			mainShardBlock: redundantChildBatch.MainShardBlock,
			childBlocks:    redundantChildBatch.ChildBlocks,
			errPredicate:   func(err error) { s.Require().ErrorContains(err, "have different length") },
		},
		{
			name:           "child hash mismatch",
			mainShardBlock: hashMismatchBatch.MainShardBlock,
			childBlocks:    hashMismatchBatch.ChildBlocks,
			errPredicate: func(err error) {
				s.Require().ErrorContains(err, "childBlocks[2].Hash != mainShardBlock.ChildBlocks[2]")
			},
		},
	}

	for _, testCase := range testCases {
		s.Run(testCase.name, func() {
			batch, err := types.NewBlockBatch(testCase.mainShardBlock, testCase.childBlocks)
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
	batch := testaide.GenerateBlockBatch(childBLockCount)

	taskEntries := batch.CreateProofTasks()

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
