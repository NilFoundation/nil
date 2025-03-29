package batches

import (
	"bytes"
	"context"
	"io"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto/kzg4844"
)

type BatchCommitter interface {
	Commit(ctx context.Context, batch *types.PrunedBatch) error
}

type batchEncoder interface {
	Encode(in *types.PrunedBatch, out io.Writer) error
}

type blobBuilder interface {
	MakeBlobs(in io.Reader, limit int) ([]kzg4844.Blob, error)
}

type ethCommitter interface {
	CommitBatch(ctx context.Context, blobs []kzg4844.Blob, batchIndex string) (*ethtypes.Transaction, error)
}

type batchCommitter struct {
	encoder      batchEncoder
	blobBuilder  blobBuilder
	ethCommitter ethCommitter
	logger       logging.Logger
	options      *commitOptions
}

func NewBatchCommitter(
	encoder batchEncoder,
	blobBuilder blobBuilder,
	ethCommitter ethCommitter,
	logger logging.Logger,
	options *commitOptions,
) BatchCommitter {
	return &batchCommitter{
		encoder:      encoder,
		blobBuilder:  blobBuilder,
		ethCommitter: ethCommitter,
		logger:       logger,
		options:      options,
	}
}

type commitOptions struct {
	maxBlobCount int
}

func DefaultCommitOptions() *commitOptions {
	return &commitOptions{
		maxBlobCount: 6,
	}
}

func (bc *batchCommitter) Commit(ctx context.Context, batch *types.PrunedBatch) error {
	var binTransactions bytes.Buffer
	if err := bc.encoder.Encode(batch, &binTransactions); err != nil {
		return err
	}
	bc.logger.Debug().Int("compressed_batch_len", binTransactions.Len()).Msg("encoded transaction")

	blobs, err := bc.blobBuilder.MakeBlobs(&binTransactions, bc.options.maxBlobCount)
	if err != nil {
		return err
	}
	bc.logger.Debug().Int("batch_blob_count", len(blobs)).Msg("packed batch blobs")

	// TODO add ethCommiter.CommitBatch() call
	bc.logger.Info().Int("blob_count", len(blobs)).Msg("committed batch")

	return nil
}
