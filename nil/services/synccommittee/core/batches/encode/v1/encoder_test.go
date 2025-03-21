package v1

import (
	"bytes"
	"encoding/binary"
	"io"
	"testing"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/services/synccommittee/core/batches/encode"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/testaide"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	scProto "github.com/NilFoundation/nil/nil/services/synccommittee/internal/types/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

type noopCompressor struct{}

func (nopc *noopCompressor) Compress(in io.Reader, out io.Writer) error {
	_, err := io.Copy(out, in)
	return err
}

func TestEncoderSimple(t *testing.T) {
	t.Parallel()

	const blockPerShard = 3
	batch := testaide.NewBlockBatch(blockPerShard)
	logger := logging.NewLogger("sc_batch_encoder_test")
	encoder := NewEncoder(logger)
	encoder.compressor = &noopCompressor{}

	prunedBatch := types.NewPrunedBatch(batch)

	var out bytes.Buffer
	err := encoder.Encode(prunedBatch, &out)
	require.NoError(t, err)

	var temp uint16

	require.NoError(t, binary.Read(&out, binary.LittleEndian, &temp))
	assert.Equal(t, encode.BatchMagic, temp)

	require.NoError(t, binary.Read(&out, binary.LittleEndian, &temp))
	assert.Equal(t, version, temp)

	var unwrappedBatch scProto.Batch
	err = proto.Unmarshal(out.Bytes(), &unwrappedBatch)
	require.NoError(t, err)

	deserializedBatch, err := ConvertFromProto(&unwrappedBatch)
	require.NoError(t, err)
	require.Len(t, deserializedBatch.Blocks, len(batch.BlockIds()))
	assert.Equal(t, batch.Id, deserializedBatch.BatchId)
	assert.ElementsMatch(t, prunedBatch.Blocks, deserializedBatch.Blocks)
}
