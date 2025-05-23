package blob

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"github.com/ethereum/go-ethereum/crypto/kzg4844"
	"github.com/icza/bitio"
)

const (
	blobSizeBytes  = len(kzg4844.Blob{})
	wordSizeBytes  = 32
	maxWordsInBlob = blobSizeBytes / wordSizeBytes

	// paddedWordSizeBits is 254 because in BLS12-381, field elements are 32 bytes (256 bits),
	// but we reserve 2 bits to ensure the number stays below the BLS12-381 field modulus,
	// making it canonical when interpreted as a big-endian integer
	paddedWordSizeBits = wordSizeBytes*8 - 2
)

type Builder interface {
	MakeBlobs(in io.Reader, limit uint) ([]kzg4844.Blob, error)
}

type builder struct{}

var _ Builder = (*builder)(nil)

func NewBuilder() Builder {
	return &builder{}
}

func (bb *builder) MakeBlobs(rd io.Reader, blobLimit uint) ([]kzg4844.Blob, error) {
	const blobSize = len(kzg4844.Blob{})

	var blobs []kzg4844.Blob
	eof := false
	writtenBits := 0
	alignedBits := 0

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

	for i := uint(0); !eof && i < blobLimit; i++ {
		blobBuf.Reset()
		wordsInBlob := 0

		blobWriter := bitio.NewWriter(&blobBuf)
		for wordsInBlob < maxWordsInBlob && !eof {
			// The first two bits of each 32-byte word are set to zero to ensure that the word is canonical
			// when it is interpreted as a big-endian big integer.
			//
			// In the context of KZG-4844, a value is considered "canonical"
			// if it is less than the BLS12-381 base field modulus.

			if err := blobWriter.WriteBits(0, 2); err != nil {
				return nil, err
			}
			alignedBits += 2

			// leadPaddingBits stores the number of padding bits needed at the start of each 254-bit word
			// to maintain proper byte alignment during read.
			// Value cycles through 0,2,4,6 as words are processed

			leadPaddingBits := byte((wordsInBlob * 2) % 8)

			err := bb.copyBits(bitReader, blobWriter, leadPaddingBits)
			if err := ignoreEOF(err); err != nil {
				return nil, err
			}

			const bytesToRead = 31
			bytesRead, err := bb.copyBytes(bitReader, blobWriter, bytesToRead)
			if err := ignoreEOF(err); err != nil {
				return nil, err
			}

			// remainingBits stores the number of bits needed to complete a 254-bit word

			remainingBits := paddedWordSizeBits - (bytesToRead*8 + leadPaddingBits)
			err = bb.copyBits(bitReader, blobWriter, remainingBits)
			if err := ignoreEOF(err); err != nil {
				return nil, err
			}

			if bytesRead < bytesToRead {
				writtenBits += bytesRead * 8
				eof = true
				break
			}

			writtenBits += paddedWordSizeBits
			wordsInBlob++
		}

		if writtenBits > 0 {
			var blob kzg4844.Blob
			copy(blob[:], blobBuf.Bytes())
			blobs = append(blobs, blob)
		}
	}
	if !eof {
		return nil, fmt.Errorf(
			"provided batch does not fit into %d blobs (%d bytes) [written = %d bits], [aligned = %d bits]",
			blobLimit, uint(blobSize)*blobLimit, writtenBits, alignedBits,
		)
	}
	return blobs, nil
}

func (*builder) copyBytes(
	reader io.Reader, writer io.Writer, bytesCount byte,
) (bytesRead int, err error) {
	if bytesCount == 0 {
		return 0, nil
	}

	buffer := make([]byte, bytesCount)

	read, err := reader.Read(buffer)
	if err != nil {
		return 0, err
	}

	if _, err := writer.Write(buffer[:read]); err != nil {
		return 0, err
	}

	return read, nil
}

func (*builder) copyBits(
	reader *bitio.Reader, writer *bitio.Writer, bitsCount byte,
) error {
	if bitsCount == 0 {
		return nil
	}

	firstBits, err := readAtMostBits(reader, bitsCount)
	if err != nil {
		return err
	}

	return writer.WriteBits(firstBits, bitsCount)
}

// readAtMostBits reads up to maxNumOfBits bits from the provided reader.
//
// Call of `readAtMostBits(reader, maxNumOfBits)` is equivalent to `reader.ReadBits(maxNumOfBits)`,
// except that the latter returns an error when the reader is exhausted.
func readAtMostBits(reader *bitio.Reader, maxNumOfBits byte) (bits uint64, err error) {
	for i := byte(0); i < maxNumOfBits; i++ {
		bit, err := reader.ReadBits(1)

		switch {
		case errors.Is(err, io.EOF):
			return bits, nil
		case err != nil:
			return 0, err
		default:
			shiftedBit := bit << (maxNumOfBits - 1 - i)
			bits |= shiftedBit
		}
	}

	return bits, nil
}
