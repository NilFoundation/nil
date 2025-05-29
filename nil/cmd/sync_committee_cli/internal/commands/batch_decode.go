package commands

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/NilFoundation/nil/nil/cmd/sync_committee_cli/internal/exec"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/services/synccommittee/core/batches/encode"
	v1 "github.com/NilFoundation/nil/nil/services/synccommittee/core/batches/encode/v1"
	"github.com/NilFoundation/nil/nil/services/synccommittee/public"
	"github.com/spf13/cobra"
)

type DecodeBatchParams struct {
	exec.NoRefreshParams

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

type decodeBatch struct {
	logger logging.Logger

	knownDecoders []batchIntermediateDecoder
	decoderLoader sync.Once
}

func NewDecodeBatchCmd(logger logging.Logger) *decodeBatch {
	return &decodeBatch{
		logger: logger,
	}
}

func (c *decodeBatch) Build() (*cobra.Command, error) {
	params := &DecodeBatchParams{}

	cmd := &cobra.Command{
		Use:   "decode-batch",
		Short: "Deserialize L1 stored batch with nil transactions into human readable format",
		RunE: func(*cobra.Command, []string) error {
			executor := exec.NewExecutor(os.Stdout, c.logger, params)
			return executor.Run(func(ctx context.Context) (exec.CmdOutput, error) {
				return c.decode(ctx, params)
			})
		},
	}

	cmd.Flags().Var(&params.BatchId, "batch-id", "unique ID of L1-stored batch")
	cmd.Flags().StringVar(
		&params.BatchFile,
		"batch-file",
		"",
		"file with binary content of concatenated blobs of the batch")
	cmd.Flags().StringVar(&params.OutputFile, "output-file", "", "target file to keep decoded batch data")

	return cmd, nil
}

type batchIntermediateDecoder interface {
	DecodeIntermediate(from io.Reader, to io.Writer) error
}

func (c *decodeBatch) initDecoders() {
	c.decoderLoader.Do(func() {
		c.knownDecoders = append(c.knownDecoders,
			v1.NewDecoder(c.logger),
			// each new implemented decoder needs to be added here
		)
	})
}

func (c *decodeBatch) decode(ctx context.Context, params *DecodeBatchParams) (exec.CmdOutput, error) {
	c.initDecoders()

	err := c.decodeAndWriteToFile(ctx, params)
	if err != nil {
		return exec.EmptyOutput, err
	}

	return "Batch is decoded successfully", nil
}

func (c *decodeBatch) decodeAndWriteToFile(ctx context.Context, params *DecodeBatchParams) (returnedErr error) {
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

		for _, decoder := range c.knownDecoders {
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
