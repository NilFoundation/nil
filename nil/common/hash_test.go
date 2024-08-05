package common

import (
	"testing"

	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/iden3/go-iden3-crypto/poseidon"
	"github.com/stretchr/testify/assert"
)

func TestPubKeyAddressShardId(t *testing.T) {
	t.Parallel()

	bytes1 := hexutil.FromHex("251e2905595df18364cf17ef0e344927e4a3dcfd24e96c9d4dc209e3421c02a5")
	bytes2 := hexutil.FromHex("0000000000000000000000000000000000000000000000000000000000000000")
	bytes1 = append(bytes1, bytes2...)
	sum := bytes1

	// Test default poseidon hash
	data := poseidon.Sum(sum)
	assert.Len(t, data, 31)

	// Test wrapper
	result := PoseidonHash(sum)
	assert.Len(t, result, 32)

	// Padding with zeros in the higher-order bytes
	assert.Equal(t, "0x00aac0fa3c5558573ba54dcb518b2f552df3f31d0483f05ac2b3f0894e9c86b5", result.Hex())
}

func TestHashNil(t *testing.T) {
	t.Parallel()

	result := PoseidonHash(nil)
	assert.Len(t, result, 32)
}
