package types

import (
	"fmt"
	"testing"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/stretchr/testify/assert"
)

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

func TestAddressFormat(t *testing.T) {
	t.Parallel()

	addr := HexToAddress("0x0002F09EC9F5cCA264eba822BB887f5c900c6e71")
	assert.Equal(t, "0x0002f09ec9f5cca264eba822bb887f5c900c6e71", fmt.Sprintf("%v", addr))
	assert.Equal(t, "0x0002f09ec9f5cca264eba822bb887f5c900c6e71", fmt.Sprintf("%s", addr))
	assert.Equal(t, "\"0x0002f09ec9f5cca264eba822bb887f5c900c6e71\"", fmt.Sprintf("%q", addr))
	assert.Equal(t, "0002f09ec9f5cca264eba822bb887f5c900c6e71", fmt.Sprintf("%x", addr))
	assert.Equal(t, "0002F09EC9F5CCA264EBA822BB887F5C900C6E71", fmt.Sprintf("%X", addr))
	assert.Equal(
		t, "[0 2 240 158 201 245 204 162 100 235 168 34 187 136 127 92 144 12 110 113]", fmt.Sprintf("%d", addr))
	assert.EqualValues(t, "0x0002f09ec9f5cca264eba822bb887f5c900c6e71", addr.hex())
	assert.Equal(t, "0x0002f09ec9f5cca264eba822bb887f5c900c6e71", addr.String())
}
