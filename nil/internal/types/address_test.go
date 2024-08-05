package types

import (
	"encoding/hex"
	"testing"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPubKeyAddressShardId(t *testing.T) {
	t.Parallel()

	shardId := ShardId(7)
	pubkey, err := hex.DecodeString("0255d1e56a49f7115b913cbd23cd68c5f471375e74a53eabeb9ca81c64a464d19f")
	require.NoError(t, err)

	addr := PubkeyBytesToAddress(shardId, pubkey)
	assert.Equal(t, shardId, addr.ShardId())
}

func TestCreateAddressShardId(t *testing.T) {
	t.Parallel()

	shardId1 := ShardId(2)
	shardId2 := ShardId(65000)

	addr1 := HexToAddress("0x0002F09EC9F5cCA264eba822BB887f5c900c6e71")
	addr2 := HexToAddress("0xfDE82e88Dc6ccABA63a4c5C23f530011c7F1A2e5")

	payload := BuildDeployPayload([]byte{12, 34}, common.EmptyHash)
	addr := CreateAddress(shardId1, payload)
	assert.Equal(t, shardId1, addr.ShardId())
	assert.Equal(t, addr, addr1)

	payload = BuildDeployPayload([]byte{56, 78}, common.EmptyHash)
	addr = CreateAddress(shardId2, payload)
	assert.Equal(t, shardId2, addr.ShardId())
	assert.Equal(t, addr, addr2)
}

func TestShardAndHexToAddress(t *testing.T) {
	t.Parallel()

	addr1 := HexToAddress("0x0002F09EC9F5cCA264eba822BB887f5c900c6e71")
	addr2 := ShardAndHexToAddress(2, "0xF09EC9F5cCA264eba822BB887f5c900c6e71")
	assert.Equal(t, addr1, addr2)

	addr1 = HexToAddress("0x0002000000000000000000000000000000000071")
	addr2 = ShardAndHexToAddress(2, "0x71")
	assert.Equal(t, addr1, addr2)

	assert.Panics(t, func() {
		ShardAndHexToAddress(2, "0x0002F09EC9F5cCA264eba822BB887f5c900c6e71")
	}, "ShardAndHexToAddress should panic on too long hex string")

	assert.Panics(t, func() {
		ShardAndHexToAddress(0x12345, "0xF09EC9F5cCA264eba822BB887f5c900c6e71")
	}, "ShardAndHexToAddress should panic on too big shard id")
}

func TestCreateRandomAddressShardId(t *testing.T) {
	t.Parallel()

	shardId1 := ShardId(2)
	shardId2 := ShardId(65000)

	addr1 := GenerateRandomAddress(shardId1)
	addr2 := GenerateRandomAddress(shardId2)

	assert.Equal(t, shardId1, addr1.ShardId())
	assert.Equal(t, shardId2, addr2.ShardId())
}
