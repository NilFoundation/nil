package batches

import (
	"testing"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/services/synccommittee/core/batches/blob"
	v1 "github.com/NilFoundation/nil/nil/services/synccommittee/core/batches/encode/v1"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/testaide"
	scTypes "github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	"github.com/stretchr/testify/suite"
)

type CommitPreparerTestSuite struct {
	suite.Suite
	preparer *commitPreparer
}

func TestCommitPreparerSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(CommitPreparerTestSuite))
}

func (s *CommitPreparerTestSuite) SetupTest() {
	logger := logging.NewLogger("commit_preparer_test")
	encoder := v1.NewEncoder(logger)
	builder := blob.NewBuilder()
	s.preparer = NewCommitPreparer(encoder, builder, DefaultCommitConfig(), logger)
}

func (s *CommitPreparerTestSuite) Test_PrepareBatchCommitment_Empty_Batch() {
	batch := scTypes.NewBlockBatch(nil, testaide.Now)

	commitment, err := s.preparer.PrepareBatchCommitment(batch)

	s.Require().Error(err)
	s.Require().ErrorContains(err, "empty batch")
	s.Require().Nil(commitment)
}

func (s *CommitPreparerTestSuite) Test_PrepareBatchCommitment_Batch_With_No_Transactions() {
	// Create a batch with blocks that have no transactions
	segments := testaide.NewChainSegments(testaide.ShardsCount)
	for _, segment := range segments {
		for _, block := range segment {
			block.Transactions = nil
		}
	}
	batch, err := scTypes.NewBlockBatch(nil, testaide.Now).WithAddedBlocks(segments, testaide.Now)
	s.Require().NoError(err)
	s.Require().NotNil(batch)

	commitment, err := s.preparer.PrepareBatchCommitment(batch)
	s.Require().NoError(err)
	s.Require().NotNil(commitment)

	s.Require().NotNil(commitment.Sidecar)
	s.Require().NotEmpty(commitment.DataProofs)
}

func (s *CommitPreparerTestSuite) Test_PrepareBatchCommitment_Batch_With_Transactions() {
	batch := testaide.NewBlockBatch(testaide.ShardsCount)

	commitment, err := s.preparer.PrepareBatchCommitment(batch)
	s.Require().NoError(err)
	s.Require().NotNil(commitment)
	s.Require().NotNil(commitment.Sidecar)
	s.Require().NotEmpty(commitment.DataProofs)

	s.Require().NotEmpty(commitment.Sidecar.Blobs)
	s.Require().NotEmpty(commitment.Sidecar.Commitments)
	s.Require().NotEmpty(commitment.Sidecar.Proofs)

	s.Require().Len(commitment.Sidecar.Blobs, len(commitment.Sidecar.Commitments))
	s.Require().Len(commitment.Sidecar.Blobs, len(commitment.Sidecar.Proofs))
}

func (s *CommitPreparerTestSuite) Test_PrepareBatchCommitment_Large_Batch() {
	batch := scTypes.NewBlockBatch(nil, testaide.Now)

	for _, seg := range testaide.NewSegmentsSequence(1000) {
		var err error
		batch, err = batch.WithAddedBlocks(seg, testaide.Now)
		s.Require().NoError(err)
	}

	commitment, err := s.preparer.PrepareBatchCommitment(batch)
	s.Require().NoError(err)
	s.Require().NotNil(commitment)
	s.Require().NotNil(commitment.Sidecar)
	s.Require().NotEmpty(commitment.DataProofs)

	s.Require().LessOrEqual(len(commitment.Sidecar.Blobs), int(s.preparer.config.MaxBlobsInTx))
}

func (s *CommitPreparerTestSuite) Test_PrepareBatchCommitment_Is_Stable() {
	batch := testaide.NewBlockBatch(testaide.ShardsCount)

	commitment, err := s.preparer.PrepareBatchCommitment(batch)
	s.Require().NoError(err)

	for range 3 {
		regenerated, err := s.preparer.PrepareBatchCommitment(batch)
		s.Require().NoError(err)
		s.Require().Equal(commitment, regenerated)
	}
}
