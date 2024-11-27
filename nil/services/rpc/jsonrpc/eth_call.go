package jsonrpc

import (
	"context"
	"errors"
	"fmt"

	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/internal/params"
	"github.com/NilFoundation/nil/nil/internal/types"
	rawapitypes "github.com/NilFoundation/nil/nil/services/rpc/rawapi/types"
	"github.com/NilFoundation/nil/nil/services/rpc/transport"
	rpctypes "github.com/NilFoundation/nil/nil/services/rpc/types"
)

// Call implements eth_call. Executes a new message call immediately without creating a transaction on the block chain.
func (api *APIImplRo) Call(ctx context.Context, args CallArgs, mainBlockNrOrHash transport.BlockNumberOrHash, overrides *StateOverrides) (*CallRes, error) {
	blockRef := rawapitypes.BlockReferenceAsBlockReferenceOrHashWithChildren(toBlockReference(mainBlockNrOrHash))
	res, err := api.rawapi.Call(ctx, args, blockRef, overrides, true)
	if err != nil {
		return nil, err
	}
	return toCallRes(res)
}

// Add some gap (20%) to be sure that it's enough for message processing.
// For now it's just heuristic function without any mathematical rationality.
func refineResult(input types.Value) types.Value {
	return input.Mul64(12).Div64(10)
}

// SstoreSentryGasEIP2200 is a requirement to execute SSTORE opcode.
// But actually we can spend less amount of gas.
// Let's try to specify reasonable upper bound for fee estimation.
const SstoreSentryGas = types.Gas(params.SstoreSentryGasEIP2200)

func refineOutMsgResult(msgs []*rpctypes.OutMessage) types.Value {
	result := types.NewZeroValue()
	if len(msgs) == 0 {
		return result
	}

	for _, msg := range msgs {
		result = result.
			Add(msg.CoinsUsed).
			Add(SstoreSentryGas.ToValue(msg.GasPrice)).
			Add(refineOutMsgResult(msg.OutMessages))
	}
	return result
}

// Call implements eth_estimateGas.
func (api *APIImplRo) EstimateFee(ctx context.Context, args CallArgs, mainBlockNrOrHash transport.BlockNumberOrHash) (types.Value, error) {
	balanceCap, err := types.NewValueFromDecimal("1000000000000000000000000") // 1 MEther
	check.PanicIfErr(err)
	feeCreditCap, err := types.NewValueFromDecimal("500000000000000000000000") // 0.5 MEther
	check.PanicIfErr(err)

	blockRef := rawapitypes.BlockReferenceAsBlockReferenceOrHashWithChildren(toBlockReference(mainBlockNrOrHash))
	execute := func(balance, feeCredit types.Value) (*rpctypes.CallResWithGasPrice, error) {
		args.FeeCredit = feeCredit

		stateOverrides := &StateOverrides{
			args.To: Contract{
				Balance: &balance,
			},
		}

		// Root message considered here as external since we anyway override contract balance.
		res, err := api.rawapi.Call(ctx, args, blockRef, stateOverrides, false)
		if err != nil {
			return nil, fmt.Errorf("failed to estimate call fee: %s", err.Error())
		}

		if res.Error != "" {
			return nil, errors.New(res.Error)
		}
		return res, nil
	}

	// Check that it's possible to run transaction with Max balance and feeCredit
	res, err := execute(balanceCap, feeCreditCap)
	if err != nil {
		return types.Value{}, err
	}

	result := res.CoinsUsed.
		Add(args.Value).
		Add(SstoreSentryGas.ToValue(res.GasPrice)).
		Add(refineOutMsgResult(res.OutMessages))

	if !args.Flags.GetBit(types.MessageFlagInternal) {
		// Heuristic price for external message verification for the wallet.
		const externalVerificationGas = types.Gas(10_000)
		result = result.Add(externalVerificationGas.ToValue(res.GasPrice))
	}
	return refineResult(result), nil
}
