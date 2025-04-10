package v1

import (
	"bytes"
	"io"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/services/synccommittee/core/batches/encode"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	"google.golang.org/protobuf/proto"
)

const version uint16 = 0x0001

type compressor interface {
	Compress(from io.Reader, to io.Writer) error
}

type batchEncoder struct {
	compressor compressor
	logger     logging.Logger
}

var _ encode.BatchEncoder = (*batchEncoder)(nil)

func NewEncoder(logger logging.Logger) *batchEncoder {
	return &batchEncoder{
		compressor: NewZstdCompressor(logger),
	}
}

func (be *batchEncoder) Encode(batch *types.PrunedBatch, out io.Writer) error {
	header := encode.NewBatchHeader(version)
	if err := header.EncodeTo(out); err != nil {
		return err
	}

	protoBatch := ConvertToProto(batch)
	be.logger.Info().Uint64("transaction_count", protoBatch.TotalTxCount).Msg("packed transactions to batch")

	serialized, err := proto.Marshal(protoBatch)
	if err != nil {
		return err
	}

	return be.compressor.Compress(bytes.NewReader(serialized), out)
}
