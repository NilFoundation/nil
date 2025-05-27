package commands

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/services/synccommittee/core/batches/encode"
	v1 "github.com/NilFoundation/nil/nil/services/synccommittee/core/batches/encode/v1"
	"github.com/NilFoundation/nil/nil/services/synccommittee/public"
)

type DecodeBatchParams struct {
	NoRefresh

	// one of
	BatchId   public.BatchId
	BatchFile string

	OutputFile string
}

func (p *DecodeBatchParams) Validate() error {
	if p.BatchId == (public.BatchId{}) && len(p.BatchFile) == 0 {
		return errors.New("either batch id or batch file must be specified")
	}

	if p.BatchId != (public.BatchId{}) && len(p.BatchFile) != 0 {
		return errors.New("only one of batch id or batch file can be specified, got both")
	}

	if len(p.BatchFile) > 0 {
		_, err := os.Stat(p.BatchFile)
		switch {
		case errors.Is(err, os.ErrNotExist):
			return fmt.Errorf("batch file %s does not exist", p.BatchFile)
		case err != nil:
			return fmt.Errorf("failed to check if batch file %s exists: %w", p.BatchFile, err)
		}
	}

	if len(p.OutputFile) == 0 {
		return errors.New("output file must be specified")
	}

	return nil
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

func DecodeBatch(ctx context.Context, params *DecodeBatchParams, logger logging.Logger) (CmdOutput, error) {
	initDecoders(logger)

	err := decodeAndWriteToFile(ctx, params)
	if err != nil {
		return EmptyOutput, err
	}

	return "Batch is decoded successfully", nil
}

func decodeAndWriteToFile(ctx context.Context, params *DecodeBatchParams) (returnedErr error) {
	var batchSource io.ReadSeeker

	var emptyBatchId public.BatchId
	if params.BatchId != emptyBatchId {
		return errors.New("fetching batch directly from the L1 is not supported yet") // TODO
	}

	if len(params.BatchFile) > 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			inFile, err := os.OpenFile(params.BatchFile, os.O_RDONLY, 0o644)
			if err != nil {
				return err
			}
			defer func(inFile *os.File) {
				returnedErr = errors.Join(returnedErr, inFile.Close())
			}(inFile)
			batchSource = inFile
		}
	}

	if batchSource == nil {
		return errors.New("batch input is not specified")
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		outFile, err := os.OpenFile(params.OutputFile, os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return err
		}
		defer func(outFile *os.File) {
			returnedErr = errors.Join(returnedErr, outFile.Close())
		}(outFile)

		for _, decoder := range knownDecoders {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				err := decoder.DecodeIntermediate(batchSource, outFile)
				if err == nil {
					return nil
				}
				if !errors.Is(err, encode.ErrInvalidVersion) {
					return err
				}

				// in case of version mismatch, reset the input stream offset and try the next available decoder
				_, err = batchSource.Seek(0, io.SeekStart)
				if err != nil {
					return err
				}
			}
		}
		return nil
	}
}
