package encode

import (
	"encoding/binary"
	"fmt"
	"io"
)

const BatchMagic uint16 = 0xDEFA

type BatchHeader struct {
	Magic   uint16
	Version uint16
}

func NewBatchHeader(version uint16) BatchHeader {
	return BatchHeader{
		Magic:   BatchMagic,
		Version: version,
	}
}

func (bh *BatchHeader) EncodeTo(out io.Writer) error {
	if err := binary.Write(out, binary.LittleEndian, bh.Magic); err != nil {
		return err
	}
	return binary.Write(out, binary.LittleEndian, bh.Version)
}

func (bh *BatchHeader) ReadFrom(in io.Reader) error {
	if err := binary.Read(in, binary.LittleEndian, &bh.Magic); err != nil {
		return err
	}
	if bh.Magic != BatchMagic {
		return fmt.Errorf("%w: read value %04X", ErrInvalidMagic, bh.Magic)
	}
	return binary.Read(in, binary.LittleEndian, &bh.Version)
}

func CheckBatchVersion(in io.Reader, desiredVersion uint16) error {
	var bh BatchHeader
	if err := bh.ReadFrom(in); err != nil {
		return err
	}
	if bh.Version != desiredVersion {
		return fmt.Errorf("%w: version is %04X", ErrInvalidVersion, bh.Version)
	}
	return nil
}
