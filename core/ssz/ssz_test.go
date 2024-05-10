package ssz

import (
	"encoding/hex"
	"testing"

	common "github.com/NilFoundation/nil/common"
	types "github.com/NilFoundation/nil/core/types"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

func TestSsz(t *testing.T) {
	block := types.Block{
		Id:                 1,
		PrevBlock:          common.Hash{0x01},
		SmartContractsRoot: common.Hash{0x02},
	}

	encoded, err := block.EncodeSSZ(nil)
	require.NoError(t, err)

	block2 := types.Block{}
	err = block2.DecodeSSZ(encoded, 0)
	require.NoError(t, err)

	require.Equal(t, block2.Id, block.Id)
	require.Equal(t, block2.PrevBlock, block.PrevBlock)
	require.Equal(t, block2.SmartContractsRoot, block.SmartContractsRoot)

	h, err := SSZHash(&block2)
	require.NoError(t, err)

	h2, err := hex.DecodeString("105d380db7f5773ffd3d99f86ef08ddd354be37962457bfdc968e739b0bea4e4")
	require.NoError(t, err)

	require.Equal(t, h, common.BytesToHash(h2))
}

func TestSszTransaction(t *testing.T) {
	message := types.Message{
		ShardInfo: types.Shard{Id: 0, GenesisBlock: common.Hash{0x01}},
		From:      common.Address{},
		To:        common.Address{},
		Value:     uint256.Int{1234},
		Data:      types.Code{0x00000001},
		Signature: common.Hash{0x02},
	}

	encoded, err := message.EncodeSSZ(nil)
	require.NoError(t, err)

	message2 := types.Message{}
	err = message2.DecodeSSZ(encoded, 0)
	require.NoError(t, err)

	require.Equal(t, message2.ShardInfo, message.ShardInfo)
	require.Equal(t, message2.From, message.From)
	require.Equal(t, message2.To, message.To)
	require.Equal(t, message2.Value, message.Value)
	require.Equal(t, message2.Data, message.Data)
	require.Equal(t, message2.Signature, message.Signature)

	h, err := SSZHash(&message2)
	require.NoError(t, err)

	h2, err := hex.DecodeString("25d61cdf3ba63fcc5f96505be3c2e5b3f5dcdfc6527216c9172f8b5def08bff1")
	require.NoError(t, err)

	require.Equal(t, h, common.BytesToHash(h2))
}
