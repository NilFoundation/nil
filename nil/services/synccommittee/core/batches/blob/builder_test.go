package blob

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMakeBlobs_ValidInput(t *testing.T) {
	t.Parallel()
	builder := NewBuilder()
	input := bytes.Repeat([]byte{0xFF}, blobSize+blobSize/2) // 1.5 blobs of 0xFF bytes
	rd := bytes.NewReader(input)

	blobs, err := builder.MakeBlobs(rd, 2)
	require.NoError(t, err)
	require.Len(t, blobs, 2)

	const u256word = 32

	fullWordWithPadding := bytes.Repeat([]byte{0xFF}, u256word)
	fullWordWithPadding[len(fullWordWithPadding)-1] = 0xFC

	// check that first blob is fully filled with data
	for word := range blobSize / u256word {
		start := word * u256word
		end := start + u256word
		require.Equal(t, fullWordWithPadding, blobs[0][start:end], "invalid data or padding word %d", word)
	}

	bytesUsedInSecondBlob := len(input) + ((len(input) / u256word) / 4)
	wordsUsedInSecondBlob := (bytesUsedInSecondBlob - blobSize) / u256word

	// check that first 2095 words of the second blob are filled with data
	for word := range wordsUsedInSecondBlob {
		start := word * u256word
		end := start + u256word
		require.Equal(t, fullWordWithPadding, blobs[1][start:end], "invalid data or padding, word %d", word)
	}

	// check that first 12 bytes of the last word are
	lastWordStart := wordsUsedInSecondBlob * u256word
	lastWordEnd := lastWordStart + 12
	require.Equal(t, blobs[1][lastWordStart:lastWordEnd], bytes.Repeat([]byte{0xFF}, 12), "invalid end of data")

	// check that the rest of the buffer is empty
	require.Equal(
		t,
		blobs[1][lastWordEnd:],
		bytes.Repeat([]byte{0x00}, blobSize-lastWordEnd),
		"end of blob is not zero padded")
}

func TestMakeBlobs_InputExceedsBlobLimit(t *testing.T) {
	t.Parallel()

	builder := NewBuilder()
	input := bytes.Repeat([]byte{0x01}, blobSize*10) // too much data to be placed into 2 blobs
	rd := bytes.NewReader(input)

	blobs, err := builder.MakeBlobs(rd, 2)
	require.Error(t, err)
	require.Nil(t, blobs)
}

func TestEmptyData(t *testing.T) {
	t.Parallel()

	builder := NewBuilder()
	var input []byte
	rd := bytes.NewReader(input)
	blobs, err := builder.MakeBlobs(rd, 2)
	require.NoError(t, err)
	assert.Empty(t, blobs)
}
