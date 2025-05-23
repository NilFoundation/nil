package blob

import (
	"bytes"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBlobReader(t *testing.T) {
	t.Parallel()
	input := make([]byte, blobSizeBytes+blobSizeBytes/2)
	for i := range input {
		input[i] = byte(i & 0xFF)
	}
	rd := bytes.NewReader(input)

	builder := NewBuilder()
	blobs, err := builder.MakeBlobs(rd, 2)
	require.NoError(t, err)
	require.Len(t, blobs, 2)

	t.Run("FullRead", func(t *testing.T) {
		t.Parallel()

		blobReader := NewReader(blobs)
		output := make([]byte, len(input))
		read, err := blobReader.Read(output)
		require.NoError(t, err)
		require.Equal(t, len(output), read)
		assert.Equal(t, input, output)
	})

	t.Run("RandomizedRead", func(t *testing.T) {
		t.Parallel()

		output := make([]byte, len(input))
		blobReader := NewReader(blobs)
		read := 0
		for read < len(output) {
			left := len(output) - read
			minRead := min(1024, left)
			maxRead := max(minRead, min(len(output)/4, left))

			readReq := rand.Intn(maxRead + 1) //nolint: gosec

			readRes, err := blobReader.Read(output[read : read+readReq])
			require.NoError(t, err)
			require.Equal(t, readReq, readRes)

			read += readRes
		}
		assert.Equal(t, input, output)
	})

	t.Run("ExcessiveRead", func(t *testing.T) {
		t.Parallel()

		blobReader := NewReader(blobs)
		output := make([]byte, 2*len(input))
		read, err := blobReader.Read(output)
		require.NoError(t, err)
		payloadInTwoBlobs := 2*blobSizeBytes - (((blobSizeBytes/32)*2)/8)*2
		require.Equal(t, payloadInTwoBlobs, read)
		assert.Equal(t, input, output[:len(input)])
	})
}
