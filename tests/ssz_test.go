package tests

import (
	"encoding/hex"
	"testing"

	common "github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/core/ssz"
	types "github.com/NilFoundation/nil/core/types"
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

	h, err := ssz.SSZHash(&block2)
	require.NoError(t, err)

	h2, err := hex.DecodeString("105d380db7f5773ffd3d99f86ef08ddd354be37962457bfdc968e739b0bea4e4")
	require.NoError(t, err)

	require.Equal(t, h, common.BytesToHash(h2))
}
