package v1

import (
	"bytes"
	"io"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/services/synccommittee/core/batches/encode"
	protoTypes "github.com/NilFoundation/nil/nil/services/synccommittee/internal/types/proto"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

type decompressor interface {
	Decompress(from io.Reader, to io.Writer) error
}

type decoder struct {
	decompressor decompressor
	logger       logging.Logger
}

func NewDecoder(logger logging.Logger) *decoder {
	return &decoder{
		decompressor: NewZstdDecompressor(logger),
		logger:       logger,
	}
}

// decodes data data from binary format into human readable
// intermediate form (transaction in proto format encoded to protojson)
// in case of need to access decoded data programmatically (from sync_committee or other cluster parts)
// this decoder might be extended with returning something like types.BlockBatch functionality
func (d *decoder) DecodeIntermediate(from io.Reader, to io.Writer) error {
	if err := encode.CheckBatchVersion(from, version); err != nil {
		return err
	}

	var decompressed bytes.Buffer
	if err := d.decompressor.Decompress(from, &decompressed); err != nil {
		return err
	}

	var protoBatch protoTypes.Batch

	if err := proto.Unmarshal(decompressed.Bytes(), &protoBatch); err != nil {
		return err
	}

	humanReadableForm, err := protojson.MarshalOptions{
		Multiline: true,
	}.Marshal(&protoBatch)
	if err != nil {
		return err
	}

	n, err := to.Write(humanReadableForm)
	if err != nil {
		return err
	}

	d.logger.Debug().Int("bytes_written", n).Str("batch_id", protoBatch.BatchId).Msg("serialized batch to protojson")
	return nil
}
