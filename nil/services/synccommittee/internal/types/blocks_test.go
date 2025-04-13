package types

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type BlocksRangeTestSuite struct {
	suite.Suite
}

func TestBlocksRange(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(BlocksRangeTestSuite))
}

func (s *BlocksRangeTestSuite) Test_SplitToChunks_Nil_Range() {
	var blocksRange *BlocksRange
	chunks := blocksRange.SplitToChunks(10)
	s.Require().Empty(chunks)
}

func (s *BlocksRangeTestSuite) Test_SplitToChunks() {
	testCases := []struct {
		name           string
		input          BlocksRange
		chunkSize      uint32
		expectedChunks []BlocksRange
	}{
		{
			name:      "Chunk_Size_Is_Equal_To_Range_Size",
			input:     NewBlocksRange(1, 10),
			chunkSize: 10,
			expectedChunks: []BlocksRange{
				NewBlocksRange(1, 10),
			},
		},
		{
			name:      "Split_Into_Equal_Chunks",
			input:     NewBlocksRange(1, 20),
			chunkSize: 5,
			expectedChunks: []BlocksRange{
				NewBlocksRange(1, 5),
				NewBlocksRange(6, 10),
				NewBlocksRange(11, 15),
				NewBlocksRange(16, 20),
			},
		},
		{
			name:      "Last_Chunk_Is_Smaller",
			input:     NewBlocksRange(1, 23),
			chunkSize: 10,
			expectedChunks: []BlocksRange{
				NewBlocksRange(1, 10),
				NewBlocksRange(11, 20),
				NewBlocksRange(21, 23),
			},
		},
		{
			name:      "Chunk_Size_Is_Larger_Than_Range",
			input:     NewBlocksRange(1, 7),
			chunkSize: 10,
			expectedChunks: []BlocksRange{
				NewBlocksRange(1, 7),
			},
		},
		{
			name:      "Single_Element_Range",
			input:     NewBlocksRange(5, 5),
			chunkSize: 2,
			expectedChunks: []BlocksRange{
				NewBlocksRange(5, 5),
			},
		},
	}

	for _, testCase := range testCases {
		s.Run(testCase.name, func() {
			actualChunks := testCase.input.SplitToChunks(testCase.chunkSize)
			s.Require().Equal(testCase.expectedChunks, actualChunks)
		})
	}
}
