package tracer

import "github.com/NilFoundation/nil/nil/common"

// getFixedSizeDataSafe returns a slice from the data based on the start and size and pads
// up to size with zero's
// This function is borrowed from EVM impl to process some opcodes
// (CODECOPY, EXTCODECOPY, CALLDATACOPY) the same way as it does
func getFixedSizeDataSafe(data []byte, start uint64, size uint64) []byte {
	length := uint64(len(data))
	if start > length {
		start = length
	}
	end := start + size
	if end > length {
		end = length
	}
	return common.RightPadBytes(data[start:end], int(size))
}

// getDataIfStartValid returns a slice from `data` starting at `start` with a length of `size` bytes.
// It returns nil if `size` is zero or if `start` is out of bounds.
// Note: It does not check if `start+size` exceeds `data` length.
func getDataIfStartValid(data []byte, start uint64, size uint64) []byte {
	if size == 0 {
		return nil
	}
	if len(data) < int(start+size) {
		return nil
	}
	return data[start : start+size]
}
