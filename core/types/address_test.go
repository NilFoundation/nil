package types

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPubKeyAddressShardId(t *testing.T) {
	t.Parallel()

	shardId := uint32(7)
	pubkey, err := hex.DecodeString("0255d1e56a49f7115b913cbd23cd68c5f471375e74a53eabeb9ca81c64a464d19f")
	require.NoError(t, err)

	addr := PubkeyBytesToAddress(shardId, pubkey)
	assert.Equal(t, shardId, addr.ShardId())
}

func TestCreateAddressShardId(t *testing.T) {
	t.Parallel()

	shardId1 := uint32(2)
	shardId2 := uint32(65000)

	addr1 := HexToAddress("0000832983856CB0CF6CD570F071122F1BEA2F20")
	addr2 := HexToAddress("1111832983856CB0CF6CD570F071122F1BEA2F20")

	addr := CreateAddress(shardId1, addr1, 123)
	assert.Equal(t, shardId1, addr.ShardId())

	addr = CreateAddress(shardId2, addr2, 456)
	assert.Equal(t, shardId2, addr.ShardId())
}

func TestCreateRandomAddressShardId(t *testing.T) {
	t.Parallel()

	shardId1 := uint32(2)
	shardId2 := uint32(65000)

	addr1 := GenerateRandomAddress(shardId1)
	addr2 := GenerateRandomAddress(shardId2)

	assert.Equal(t, shardId1, addr1.ShardId())
	assert.Equal(t, shardId2, addr2.ShardId())
}
