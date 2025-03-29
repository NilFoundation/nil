package execution

import (
	"sync"

	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/shopspring/decimal"
)

var lock sync.Mutex

type FeeCalculator interface {
	CalculateBaseFee(prevBlock *types.Block) types.Value
}

type ConstFeeCalculator struct {
	Value types.Value
}

func (c *ConstFeeCalculator) CalculateBaseFee(prevBlock *types.Block) types.Value {
	return c.Value
}

type MainFeeCalculator struct{}

func GetEffectivePriorityFee(baseFee types.Value, txn *types.Transaction) (types.Value, bool) {
	effectivePriorityFee, overflow := txn.MaxFeePerGas.SubOverflow(baseFee)
	if overflow {
		return types.Value0, false
	}
	if effectivePriorityFee.Cmp(txn.MaxPriorityFeePerGas) > 0 {
		effectivePriorityFee = txn.MaxPriorityFeePerGas
	}
	return effectivePriorityFee, true
}

func GasTarget(gasLimit types.Gas) types.Gas {
	return gasLimit / 2
}

var (
	// Multiplier for base fee adjustment
	adjustmentFactor = decimal.New(1, 0).Div(decimal.New(1013400, -4))
	// Gas limit normalized to percentage (100% = 100.0)
	gasLimitPercentage = decimal.New(1000000, -4)
	// Smoothing factor for sigmoids
	smoothingFactor = decimal.New(50000, -4)
	// Center of first sigmoid (25% of gas limit)
	centerSigmoid1 = decimal.New(2500, -4).Mul(gasLimitPercentage)
	// Center of second sigmoid (75% of gas limit)
	centerSigmoid2 = decimal.New(7500, -4).Mul(gasLimitPercentage)
	blockGasLimit  = decimal.NewFromInt(int64(types.DefaultMaxGasInBlock.Uint64()))
	maxSigmaDiff   decimal.Decimal
	eNumber        = decimal.New(27182818284, -10)
)

func init() {
	a := sigmoid2(gasLimitPercentage, centerSigmoid2, smoothingFactor).
		Sub(sigmoid1(gasLimitPercentage, centerSigmoid1, smoothingFactor))
	b := sigmoid2(decimal.New(0, -4), centerSigmoid2, smoothingFactor).
		Sub(sigmoid1(decimal.New(0, -4), centerSigmoid1, smoothingFactor))
	if a.Cmp(b) >= 0 {
		maxSigmaDiff = a
	} else {
		maxSigmaDiff = b
	}
}

func (m *MainFeeCalculator) CalculateBaseFee(prevBlock *types.Block) types.Value {
	gasUsedPrevious := prevBlock.GasUsed
	baseFeePrevious := prevBlock.BaseFee

	// Convert to percentage
	gasUsedPercentage := toPercentage(decimal.NewFromInt(int64(gasUsedPrevious.Uint64())), blockGasLimit)

	// Calculate the difference between the two sigmoids
	sigmaDiff := sigmoid2(gasUsedPercentage, centerSigmoid2, smoothingFactor).Sub(
		sigmoid1(gasUsedPercentage, centerSigmoid1, smoothingFactor))

	// Normalize the difference
	normalized := sigmaDiff.Div(maxSigmaDiff)

	baseFeePreviousFixed := decimal.NewFromBigInt(baseFeePrevious.Int().ToBig(), 0)
	newFee := baseFeePreviousFixed.Mul(adjustmentFactor.Mul(normalized).Add(decimal.New(1, 0)))

	resultBaseFee := types.DefaultGasPrice
	if newFee.Cmp(decimal.NewFromBigInt(types.DefaultGasPrice.ToBig(), 0)) > 0 {
		resultBaseFee = types.NewValueFromBigMust(newFee.BigInt())
	}

	return resultBaseFee
}

func toPercentage(gasUsedPrevious, gasLimitAbsolute decimal.Decimal) decimal.Decimal {
	return gasUsedPrevious.Mul(decimal.New(100, 0)).Div(gasLimitAbsolute)
}

func sigmoid1(gasUsedPercentage, centerSigmoid, smoothingFactor decimal.Decimal) decimal.Decimal {
	n := gasUsedPercentage.Sub(centerSigmoid).Div(smoothingFactor)
	// Decimal's Pow function is not thread-safe, so we need to lock it
	// https://github.com/shopspring/decimal/issues/368
	// TODO: This is a workaround, we should find a better solution
	lock.Lock()
	exp := eNumber.Pow(n)
	lock.Unlock()
	return decimal.NewFromInt(1).Div(decimal.NewFromInt(1).Add(exp))
}

func sigmoid2(gasUsedPercentage, centerSigmoid, smoothingFactor decimal.Decimal) decimal.Decimal {
	n := gasUsedPercentage.Sub(centerSigmoid).Neg().Div(smoothingFactor)
	// Decimal's Pow function is not thread-safe, so we need to lock it
	// https://github.com/shopspring/decimal/issues/368
	// TODO: This is a workaround, we should find a better solution
	lock.Lock()
	exp := eNumber.Pow(n)
	lock.Unlock()
	return decimal.NewFromInt(1).Div(decimal.NewFromInt(1).Add(exp))
}
