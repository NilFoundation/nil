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
	f := feeCalc.CalculateBaseFee(prevBlock)
	require.Equal(t, prevBlock.BaseFee.Uint64(), f.Uint64())

	prevBlock.BaseFee = f
	prevBlock.GasUsed = gasTarget * 2
	f = feeCalc.CalculateBaseFee(prevBlock)
	require.Greater(t, f.Uint64(), prevBlock.BaseFee.Uint64())

	prevBlock.BaseFee = f
	prevBlock.GasUsed = gasTarget - 1_000_000
	f = feeCalc.CalculateBaseFee(prevBlock)
	require.Less(t, f.Uint64(), prevBlock.BaseFee.Uint64())
}

func TestEffectivePriorityFee(t *testing.T) {
	t.Parallel()

	tx := types.NewEmptyTransaction()
	tx.MaxPriorityFeePerGas = types.NewValueFromUint64(100)
	tx.MaxFeePerGas = types.NewValueFromUint64(1200)
	baseFee := types.NewValueFromUint64(1000)
	eff, ok := GetEffectivePriorityFee(baseFee, tx)
	require.True(t, ok)
	require.Equal(t, 0, eff.Cmp(types.NewValueFromUint64(100)))

	tx.MaxPriorityFeePerGas = types.NewValueFromUint64(100)
	tx.MaxFeePerGas = types.NewValueFromUint64(1050)
	baseFee = types.NewValueFromUint64(1000)
	eff, ok = GetEffectivePriorityFee(baseFee, tx)
	require.True(t, ok)
	require.Equal(t, 0, eff.Cmp(types.NewValueFromUint64(50)))

	tx.MaxPriorityFeePerGas = types.NewValueFromUint64(100)
	tx.MaxFeePerGas = types.NewValueFromUint64(1100)
	baseFee = types.NewValueFromUint64(1101)
	eff, ok = GetEffectivePriorityFee(baseFee, tx)
	require.False(t, ok)
	require.True(t, eff.IsZero())
}
