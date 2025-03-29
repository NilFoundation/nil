package blob

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"github.com/ethereum/go-ethereum/crypto/kzg4844"
	"github.com/icza/bitio"
)

const blobSize = len(kzg4844.Blob{})

type builder struct{}

func NewBuilder() *builder {
	return &builder{}
}

func (bb *builder) MakeBlobs(rd io.Reader, blobLimit int) ([]kzg4844.Blob, error) {
	const blobSize = len(kzg4844.Blob{})

	var blobs []kzg4844.Blob
	eof := false
	writtenBits := 0

	bitReader := bitio.NewReader(rd) // bit wrapper for reading 254-bit pieces of data to place into the blobs

	var blobBuf bytes.Buffer
	blobBuf.Grow(blobSize)

	ignoreEOF := func(err error) error {
		if errors.Is(err, io.EOF) {
			eof = true
			return nil
		}
		return err
	}

	align := 0
	for i := 0; !eof && i < blobLimit; i++ {
		blobBuf.Reset()

		blobWriter := bitio.NewWriter(&blobBuf)
		writtenInBlob := 0
		for writtenInBlob < blobSize && !eof {
			var ethWordBuf [31]byte

			read, err := bitReader.Read(ethWordBuf[:])
			if err := ignoreEOF(err); err != nil {
				return nil, err
			}

			wr, err := blobWriter.Write(ethWordBuf[:read])
			if err != nil {
				return nil, err
			}
			writtenInBlob += wr

			if read < len(ethWordBuf) {
				writtenBits += read * 8
				eof = true
				break
			}

			const lastByteBits = 6 // each 2 last bits of every u256 word cannot be used

			lastbyte, err := bitReader.ReadBits(lastByteBits)
			if err := ignoreEOF(err); err != nil {
				return nil, err
			}

			if err := blobWriter.WriteBits(lastbyte, lastByteBits); err != nil {
				return nil, err
			}

			aligned, err := blobWriter.Align()
			if err != nil {
				return nil, err
			}
			align += int(aligned)
			writtenInBlob++

			writtenBits += 32*8 - 2
		}

		if writtenBits > 0 {
			var blob kzg4844.Blob
			copy(blob[:], blobBuf.Bytes())
			blobs = append(blobs, blob)
		}
	}
	if !eof {
		return nil, fmt.Errorf(
			"provided batch does not fit into %d blobs (%d bytes) [written = %d bits] [align = %d bits]",
			blobLimit, blobSize*blobLimit, writtenBits, align)
	}
	return blobs, nil
}
