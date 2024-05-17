package tests

import (
	"encoding/hex"
	"testing"

	common "github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/core/ssz"
	types "github.com/NilFoundation/nil/core/types"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

func TestSszBlock(t *testing.T) {
	block := types.Block{
		Id:                 1,
		PrevBlock:          common.Hash{0x01},
		SmartContractsRoot: common.Hash{0x02},
	}

	encoded, err := block.MarshalSSZ()
	require.NoError(t, err)

	block2 := types.Block{}
	err = block2.UnmarshalSSZ(encoded)
	require.NoError(t, err)

	require.Equal(t, block2.Id, block.Id)
	require.Equal(t, block2.PrevBlock, block.PrevBlock)
	require.Equal(t, block2.SmartContractsRoot, block.SmartContractsRoot)

	h, err := ssz.FastSSZHash(&block2)
	require.NoError(t, err)

	h2, err := hex.DecodeString("105d380db7f5773ffd3d99f86ef08ddd354be37962457bfdc968e739b0bea4e4")
	require.NoError(t, err)

	require.Equal(t, h, common.BytesToHash(h2))
}

func TestSszMessage(t *testing.T) {
	message := types.Message{
		ShardId:   types.MasterShardId,
		From:      common.Address{},
		To:        common.Address{},
		Value:     uint256.Int{1234},
		Data:      types.Code{0x00000001},
		Seqno:     567,
		Signature: common.Hash{0x02},
	}

	encoded, err := message.EncodeSSZ(nil)
	require.NoError(t, err)

	message2 := types.Message{}
	err = message2.DecodeSSZ(encoded, 0)
	require.NoError(t, err)

	require.Equal(t, message2.ShardId, message.ShardId)
	require.Equal(t, message2.From, message.From)
	require.Equal(t, message2.To, message.To)
	require.Equal(t, message2.Value, message.Value)
	require.Equal(t, message2.Data, message.Data)
	require.Equal(t, message2.Seqno, message.Seqno)
	require.Equal(t, message2.Signature, message.Signature)

	h, err := ssz.SSZHash(&message2)
	require.NoError(t, err)

	h2, err := hex.DecodeString("0219b023e05f696ef3f97591529c4d0e5b02b8dbfb2a7ed4f13949091e78c176")
	require.NoError(t, err)

	require.Equal(t, common.BytesToHash(h2), h)
}

func TestSszSmc(t *testing.T) {
	smc := types.SmartContract{
		Address:     common.HexToAddress("1d9bc16f1a559"),
		Initialised: true,
		Balance:     uint256.Int{1234},
		StorageRoot: common.Hash{0x01},
		CodeHash:    common.Hash{0x02},
		Seqno:       567,
	}

	encoded, err := smc.EncodeSSZ(nil)
	require.NoError(t, err)

	smc2 := types.SmartContract{}
	err = smc2.DecodeSSZ(encoded, 0)
	require.NoError(t, err)

	require.Equal(t, smc.Address, smc2.Address)
	require.Equal(t, smc.Initialised, smc2.Initialised)
	require.Equal(t, smc.Balance, smc2.Balance)
	require.Equal(t, smc.StorageRoot, smc2.StorageRoot)
	require.Equal(t, smc.CodeHash, smc2.CodeHash)
	require.Equal(t, smc.Seqno, smc2.Seqno)

	h, err := ssz.SSZHash(&smc2)
	require.NoError(t, err)

	h2, err := hex.DecodeString("1d153492db725f651b7c3ef0caf9aaa01bc4820ed2e1b9e0cb30636325f0d818")
	require.NoError(t, err)

	require.Equal(t, common.BytesToHash(h2), h)
}
