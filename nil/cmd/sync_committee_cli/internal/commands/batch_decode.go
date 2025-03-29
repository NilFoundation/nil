package commands

import (
	"context"
	"errors"
	"io"
	"os"
	"sync"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/services/synccommittee/core/batches/encode"
	v1 "github.com/NilFoundation/nil/nil/services/synccommittee/core/batches/encode/v1"
	"github.com/NilFoundation/nil/nil/services/synccommittee/public"
)

type DecodeBatchParams struct {
	// one of
	BatchId   public.BatchId
	BatchFile string

	OutputFile string
}

type batchIntermediateDecoder interface {
	DecodeIntermediate(from io.Reader, to io.Writer) error
}

var (
	knownDecoders []batchIntermediateDecoder
	decoderLoader sync.Once
)

func initDecoders(logger logging.Logger) {
	decoderLoader.Do(func() {
		knownDecoders = append(knownDecoders,
			v1.NewDecoder(logger),
			// each new implemented decoder needs to be added here
		)
	})
}

// TODO embed this call into commands.Executor?
func DecodeBatch(_ context.Context, params *DecodeBatchParams, logger logging.Logger) error {
	initDecoders(logger)

	var batchSource io.ReadSeeker

	var emptyBatchId public.BatchId
	if params.BatchId != emptyBatchId {
		return errors.New("fetching batch directly from the L1 is not supported yet") // TODO
	}

	if len(params.BatchFile) > 0 {
		inFile, err := os.OpenFile(params.BatchFile, os.O_RDONLY, 0o644)
		if err != nil {
			return err
		}
		defer inFile.Close()
		batchSource = inFile
	}

	if batchSource == nil {
		return errors.New("batch input is not specified")
	}

	outFile, err := os.OpenFile(params.OutputFile, os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}

	for _, decoder := range knownDecoders {
		err := decoder.DecodeIntermediate(batchSource, outFile)
		if err == nil {
			break
		}
		if !errors.Is(err, encode.ErrInvalidVersion) {
			return err
		}

		// in case of version mismatch reset the input stream offset and try next available decoder
		_, err = batchSource.Seek(0, io.SeekStart)
		if err != nil {
			return err
		}
	}
	return nil
}
