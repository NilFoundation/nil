package ssz

import (
	"encoding/binary"

	common "github.com/NilFoundation/nil/common"
	"github.com/holiman/uint256"
	"github.com/iden3/go-iden3-crypto/poseidon"
)

var (
	BaseExtraDataSSZOffsetHeader = 536
	BaseExtraDataSSZOffsetBlock  = 508
)

func MarshalUint64SSZ(buf []byte, x uint64) {
	binary.LittleEndian.PutUint64(buf, x)
}

func Uint64SSZ(x uint64) []byte {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, x)
	return b
}

func Uint256SSZ(x uint256.Int) []byte {
	b := make([]byte, 32)
	x.WriteToSlice(b)
	return b
}

func BoolSSZ(b bool) byte {
	if b {
		return 1
	}
	return 0
}

func OffsetSSZ(x uint32) []byte {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, x)
	return b
}

// EncodeOffset marshals a little endian uint32 to buf
func EncodeOffset(buf []byte, offset uint32) {
	binary.LittleEndian.PutUint32(buf, offset)
}

// ReadOffset unmarshals a little endian uint32 to dst
func DecodeOffset(x []byte) uint32 {
	return binary.LittleEndian.Uint32(x)
}

func UnmarshalUint64SSZ(x []byte) uint64 {
	return binary.LittleEndian.Uint64(x)
}

func UnmarshalUint256SSZ(s []byte) uint256.Int {
	var res uint256.Int
	res.SetBytes(s)
	return res
}

func DecodeDynamicList[T SSZDecodable](bytes []byte, start, end uint32, max uint64, version int) ([]T, error) {
	if start > end || len(bytes) < int(end) {
		return nil, ErrBadOffset
	}
	buf := bytes[start:end]
	var elementsNum, currentOffset uint32
	if len(buf) > 4 {
		currentOffset = DecodeOffset(buf)
		elementsNum = currentOffset / 4
	}
	inPos := 4
	if uint64(elementsNum) > max {
		return nil, ErrTooBigList
	}
	objs := make([]T, elementsNum)
	for i := range objs {
		endOffset := uint32(len(buf))
		if i != len(objs)-1 {
			if len(buf[inPos:]) < 4 {
				return nil, ErrLowBufferSize
			}
			endOffset = DecodeOffset(buf[inPos:])
		}
		inPos += 4
		if endOffset < currentOffset || len(buf) < int(endOffset) {
			return nil, ErrBadOffset
		}
		objs[i] = objs[i].Clone().(T)
		if err := objs[i].DecodeSSZ(buf[currentOffset:endOffset], version); err != nil {
			return nil, err
		}
		currentOffset = endOffset
	}
	return objs, nil
}

func DecodeStaticList[T SSZDecodable](bytes []byte, start, end, bytesPerElement uint32, max uint64, version int) ([]T, error) {
	if start > end || len(bytes) < int(end) {
		return nil, ErrBadOffset
	}
	buf := bytes[start:end]
	elementsNum := uint64(len(buf)) / uint64(bytesPerElement)
	// Check for errors
	if uint32(len(buf))%bytesPerElement != 0 {
		return nil, ErrBufferNotRounded
	}
	if elementsNum > max {
		return nil, ErrTooBigList
	}
	objs := make([]T, elementsNum)
	for i := range objs {
		objs[i] = objs[i].Clone().(T)
		if err := objs[i].DecodeSSZ(buf[i*int(bytesPerElement):], version); err != nil {
			return nil, err
		}
	}
	return objs, nil
}

func DecodeHashList(bytes []byte, start, end, max uint32) ([]common.Hash, error) {
	if start > end || len(bytes) < int(end) {
		return nil, ErrBadOffset
	}
	buf := bytes[start:end]
	elementsNum := uint32(len(buf)) / common.HashSize
	// Check for errors
	if uint32(len(buf))%common.HashSize != 0 {
		return nil, ErrBufferNotRounded
	}
	if elementsNum > max {
		return nil, ErrTooBigList
	}
	objs := make([]common.Hash, elementsNum)
	for i := range objs {
		copy(objs[i][:], buf[i*common.HashSize:])
	}
	return objs, nil
}

func DecodeNumbersList(bytes []byte, start, end uint32, max uint64) ([]uint64, error) {
	if start > end || len(bytes) < int(end) {
		return nil, ErrBadOffset
	}
	buf := bytes[start:end]
	elementsNum := uint64(len(buf)) / common.BlockNumSize
	// Check for errors
	if uint64(len(buf))%common.BlockNumSize != 0 {
		return nil, ErrBufferNotRounded
	}
	if elementsNum > max {
		return nil, ErrTooBigList
	}
	objs := make([]uint64, elementsNum)
	for i := range objs {
		objs[i] = UnmarshalUint64SSZ(buf[i*common.BlockNumSize:])
	}
	return objs, nil
}

func CalculateIndiciesLimit(maxCapacity, numItems, size uint64) uint64 {
	limit := (maxCapacity*size + 31) / 32
	if limit != 0 {
		return limit
	}
	if numItems == 0 {
		return 1
	}
	return numItems
}

func DecodeString(bytes []byte, start, end, max uint64) ([]byte, error) {
	if start > end || len(bytes) < int(end) {
		return nil, ErrBadOffset
	}
	buf := bytes[start:end]
	if uint64(len(buf)) > max {
		return nil, ErrTooBigList
	}
	return buf, nil
}

func EncodeDynamicList[T SSZEncodable](buf []byte, objs []T) (dst []byte, err error) {
	dst = buf
	// Attestation
	subOffset := len(objs) * 4
	for _, attestation := range objs {
		dst = append(dst, OffsetSSZ(uint32(subOffset))...)
		subOffset += attestation.EncodingSizeSSZ()
	}
	for _, obj := range objs {
		dst, err = obj.EncodeSSZ(dst)
		if err != nil {
			return
		}
	}
	return
}

func SSZHash(obj SSZEncodable) (common.Hash, error) {
	encoded, err := obj.EncodeSSZ(nil)
	if err != nil {
		return common.Hash{0}, err
	}
	return common.BytesToHash(poseidon.Sum(encoded)), nil
}
