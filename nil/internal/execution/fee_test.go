package execution

import (
	"testing"

	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/stretchr/testify/require"
)

func TestPriceCalculation(t *testing.T) {
	t.Parallel()

	feeCalc := MainFeeCalculator{}

	prevBlock := &types.Block{}

	gasTarget := GasTarget(types.DefaultMaxGasInBlock)
	prevBlock.BaseFee = types.DefaultGasPrice
	prevBlock.GasUsed = gasTarget
	f, err := feeCalc.CalculateBaseFee(prevBlock)
	require.NoError(t, err)
	require.Equal(t, prevBlock.BaseFee.Uint64(), f.Uint64())

	prevBlock.BaseFee = f
	prevBlock.GasUsed = gasTarget * 2
	f, err = feeCalc.CalculateBaseFee(prevBlock)
	require.NoError(t, err)
	require.Greater(t, f.Uint64(), prevBlock.BaseFee.Uint64())

	prevBlock.BaseFee = f
	prevBlock.GasUsed = gasTarget - 1_000_000
	f, err = feeCalc.CalculateBaseFee(prevBlock)
	require.NoError(t, err)
	require.Less(t, f.Uint64(), prevBlock.BaseFee.Uint64())
}
