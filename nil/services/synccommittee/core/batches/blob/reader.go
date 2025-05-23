package blob

import (
	"bytes"
	"fmt"
	"io"

	"github.com/ethereum/go-ethereum/crypto/kzg4844"
	"github.com/icza/bitio"
)

type reader struct {
	blobs        []kzg4844.Blob
	wordOffset   int
	blobOffset   int
	curBlobIdx   int
	curBitReader *bitio.Reader
}

var _ io.Reader = (*reader)(nil)

func NewReader(blobs []kzg4844.Blob) *reader {
	r := &reader{blobs: blobs}
	if len(r.blobs) > 0 {
		r.curBitReader = bitio.NewReader(bytes.NewReader(r.blobs[0][:]))
	}
	return r
}

func (r *reader) Read(dst []byte) (int, error) {
	dstBits := len(dst) * 8
	var buf bytes.Buffer
	writer := bitio.NewWriter(&buf)
	writtenBits := 0

	// read by 64bit chunks with 254bit alignment
	// could be optimized to use underlying io.Reader but it seems to be rarely used
	// and complicates the code
	for !r.eof() && writtenBits < dstBits {
		if r.wordOffset == 0 {
			if _, err := r.readBits(2); err != nil {
				return writtenBits / 8, err
			}
			r.wordOffset += 2
		}

		availableOutputBits := dstBits - writtenBits
		toRead := uint8(min(availableOutputBits, 64, 256-r.wordOffset))
		bits, err := r.readBits(toRead)
		if err != nil {
			return writtenBits / 8, err
		}

		r.wordOffset += int(toRead)

		if err := writer.WriteBits(bits, toRead); err != nil {
			return writtenBits / 8, err
		}
		writtenBits += int(toRead)

		r.wordOffset %= 256
	}
	copy(dst, buf.Bytes()) // bytes.NewBuffer is an owning call so it is potentially unsafe to use dst without copying

	return writtenBits / 8, nil
}

func (r *reader) readBits(n uint8) (uint64, error) {
	const blobBitSize = blobSizeBytes * 8

	if r.eof() {
		return 0, io.EOF
	}
	newOffset := r.blobOffset + int(n)
	if newOffset > blobBitSize {
		return 0, fmt.Errorf("not aligned blob read is not permitted (current %d requested %d)", r.blobOffset, n)
	}

	ret, err := r.curBitReader.ReadBits(n)
	if err != nil {
		return ret, err
	}
	r.blobOffset = newOffset
	if r.blobOffset >= blobBitSize {
		r.advance()
	}
	return ret, err
}

func (r *reader) advance() bool {
	if r.curBlobIdx < len(r.blobs) {
		r.curBlobIdx++
	}
	r.curBitReader = nil
	if r.eof() {
		return false
	}
	r.curBitReader = bitio.NewReader(bytes.NewReader(r.blobs[r.curBlobIdx][:]))
	r.blobOffset = 0
	return true
}

func (r *reader) eof() bool {
	return r.curBlobIdx >= len(r.blobs)
}
