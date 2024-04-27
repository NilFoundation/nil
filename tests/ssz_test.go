package tests

import (
	common "github.com/NilFoundation/nil/common"
	types "github.com/NilFoundation/nil/core/types"
	"github.com/stretchr/testify/require"
	"testing"
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
}
