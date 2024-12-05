package tracer

import "github.com/NilFoundation/nil/nil/common"

// getDataOverflowSafe returns a slice from the data based on the start and size and pads
// up to size with zero's
// This function is borrowed from EVM impl to process some opcodes
// (CODECOPY, EXTCODECOPY, CALLDATACOPY) the same way as it does
func getDataOverflowSafe(data []byte, start uint64, size uint64) []byte {
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
