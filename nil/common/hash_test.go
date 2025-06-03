package common

import (
	"testing"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
)

func TestPubKeyAddressShardId(t *testing.T) {
	t.Parallel()

	bytes := ethcommon.FromHex("251e2905595df18364cf17ef0e344927e4a3dcfd24e96c9d4dc209e3421c02a5" +
		"0000000000000000000000000000000000000000000000000000000000000000")

	// Test wrapper
	result := KeccakHash(bytes)
	assert.Len(t, result, 32)

	// Padding with zeros in the higher-order bytes
	assert.Equal(t, "0x79cc8d0ed0bc0b6600a8131e7083a285db6dfcb850736f3a67f765dc8f628504", result.Hex())
}

func TestHashNil(t *testing.T) {
	t.Parallel()

	result := KeccakHash(nil)
	assert.Len(t, result, 32)
}
