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
	input := bytes.Repeat([]byte{0xFF}, blobSizeBytes+blobSizeBytes/2) // 1.5 blobs of 0xFF bytes
	rd := bytes.NewReader(input)

	blobs, err := builder.MakeBlobs(rd, 2)
	require.NoError(t, err)
	require.Len(t, blobs, 2)

	const u256word = 32
	const paddedByte = 0b00111111

	fullWordWithPadding := bytes.Repeat([]byte{0xFF}, u256word)
	fullWordWithPadding[0] = paddedByte

	// check that first blob is fully filled with data
	for word := range blobSizeBytes / u256word {
		start := word * u256word
		end := start + u256word
		require.Equal(t, fullWordWithPadding, blobs[0][start:end], "invalid data or padding word %d", word)
	}

	bytesUsedInSecondBlob := len(input) + ((len(input) / u256word) / 4)
	wordsUsedInSecondBlob := (bytesUsedInSecondBlob - blobSizeBytes) / u256word
	// check that first 2095 words of the second blob are filled with data
	for word := range wordsUsedInSecondBlob {
		start := word * u256word
		end := start + u256word
		require.Equal(t, fullWordWithPadding, blobs[1][start:end], "invalid data or padding, word %d", word)
	}

	lastWordStart := wordsUsedInSecondBlob * u256word
	lastWordEnd := lastWordStart + 13
	lastWordActual := blobs[1][lastWordStart:lastWordEnd]

	// check that the last word in the second blobs contains 12 bytes of padded data

	lastWordExpected := bytes.Repeat([]byte{0xFF}, 13)
	lastWordExpected[0] = paddedByte
	lastWordExpected[12] = 0b1100_0000

	require.Equal(t, lastWordExpected, lastWordActual, "invalid end of data")

	// check that the rest of the buffer is empty
	require.Equal(
		t,
		blobs[1][lastWordEnd:],
		bytes.Repeat([]byte{0x00}, blobSizeBytes-lastWordEnd),
		"end of blob is not zero padded",
	)
}

// TestMakeBlobs_Small_Input verifies the creation of blobs from a "small" input
// and checks alignment and padding correctness.
//
// Input data:
// 0b11111111 {33 bytes}
//
// Output blob:
// Idx      i=00        i=01              i=31        i=32        i=33        i=34
// Byte  0b00111111, 0b11111111, ...,  0b11111111, 0b00111111, 0b11110000, 0b00000000, ...
func Test_MakeBlobs_Small_Input(t *testing.T) {
	t.Parallel()

	builder := NewBuilder()
	const inputByte = byte(0b11111111)
	input := bytes.Repeat([]byte{inputByte}, 33)
	bytesReader := bytes.NewReader(input)

	blobs, err := builder.MakeBlobs(bytesReader, 1)
	require.NoError(t, err)
	require.Len(t, blobs, 1)
	blob := blobs[0]

	const paddedByte = byte(0b00111111)
	require.Equal(t, paddedByte, blob[0])

	for i := 1; i <= 31; i++ {
		require.Equal(t, inputByte, blob[i])
	}

	require.Equal(t, paddedByte, blob[32])

	const lastByte = byte(0b11110000)
	require.Equal(t, lastByte, blob[33])

	for _, tailByte := range blob[34:] {
		require.Equal(t, byte(0x00), tailByte)
	}
}

func TestMakeBlobs_InputExceedsBlobLimit(t *testing.T) {
	t.Parallel()

	builder := NewBuilder()
	input := bytes.Repeat([]byte{0x01}, blobSizeBytes*10) // too much data to be placed into 2 blobs
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
