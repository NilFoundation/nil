package v1

import (
	"io"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/klauspost/compress/zstd"
)

// some aux interfaces for operating on memory buffers
type LenAware interface {
	Len() int
}

type Growable interface {
	Grow(int)
}

type zstdCompressor struct {
	logger logging.Logger
}

func NewZstdCompressor(logger logging.Logger) *zstdCompressor {
	return &zstdCompressor{
		logger: logger,
	}
}

func (zc *zstdCompressor) Compress(from io.Reader, to io.Writer) (err error) {
	impl, err := zstd.NewWriter(to, zstd.WithEncoderLevel(zstd.SpeedBetterCompression))
	if err != nil {
		return err
	}

	defer func() {
		closeErr := impl.Close()
		if err == nil {
			err = closeErr
		}
	}()

	var maxEstimatedSize int
	if lenAware, ok := from.(LenAware); ok {
		maxEstimatedSize = lenAware.Len()
		zc.logger.Trace().Int("estimated_size", maxEstimatedSize).Send()
	}

	// adjust the sizes if we are operating with memory buffers
	if growable, ok := to.(Growable); maxEstimatedSize > 0 && ok {
		growable.Grow(maxEstimatedSize)
	}

	_, err = impl.ReadFrom(from)
	return
}

type zstdDecompressor struct {
	logger logging.Logger
}

func NewZstdDecompressor(logger logging.Logger) *zstdDecompressor {
	return &zstdDecompressor{
		logger: logger,
	}
}

func (zd *zstdDecompressor) Decompress(in io.Reader, out io.Writer) error {
	impl, err := zstd.NewReader(in)
	if err != nil {
		return err
	}
	defer impl.Close()

	n, err := impl.WriteTo(out)
	if err != nil {
		return err
	}

	zd.logger.Info().Int64("decompressed_size", n).Msg("decomressed zstd batch")
	return nil
}
