package jsonrpc

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/internal/vm"
	"github.com/NilFoundation/nil/nil/services/rpc/transport"
	rpctypes "github.com/NilFoundation/nil/nil/services/rpc/types"
)

// Call implements eth_call. Executes a new message call immediately without creating a transaction on the block chain.
func (api *APIImpl) Call(ctx context.Context, args CallArgs, mainBlockNrOrHash transport.BlockNumberOrHash, overrides *StateOverrides) (*CallRes, error) {
	blockRef := toBlockReference(mainBlockNrOrHash)
	res, err := api.rawapi.Call(ctx, args, blockRef, overrides, true)
	if err != nil {
		return nil, err
	}
	return toCallRes(res)
}

func feeIsNotEnough(err error) bool {
	if err == nil {
		return false
	}

	msg := err.Error()
	return strings.HasPrefix(msg, vm.ErrOutOfGas.Error()) ||
		strings.HasPrefix(err.Error(), vm.ErrInsufficientBalance.Error())
}

// Add some gap (20%) to be sure that it's enough for message processing.
// For now it's just heuristic function without any mathematical rationality.
func refineResult(input types.Value) types.Value {
	return input.Mul64(12).Div64(10)
}

// Call implements eth_estimateGas.
func (api *APIImpl) EstimateFee(ctx context.Context, args CallArgs, mainBlockNrOrHash transport.BlockNumberOrHash) (types.Value, error) {
	gasCap := types.NewValueFromUint64(100_000_000)
	feeCreditCap := types.NewValueFromUint64(50_000_000)

	blockRef := toBlockReference(mainBlockNrOrHash)
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
	res, err := execute(gasCap, feeCreditCap)
	if err != nil {
		return types.Value{}, err
	}

	var lo, hi types.Value = res.CoinsUsed.Add(args.Value), feeCreditCap

	// Binary search implementation.
	// Some opcodes can require some gas reserve (so we can't use coinsUsed).
	// As result we try to find minimal value then call works successful.
	// E.g. see how "SstoreSentryGasEIP2200" is used.
	for lo.Add64(1).Int().Lt(hi.Int()) {
		mid := hi.Add(lo).Div64(2)

		_, err = execute(gasCap, mid)
		switch {
		case err == nil:
			hi = mid
		case feeIsNotEnough(err):
			lo = mid
		default:
			return types.Value{}, err
		}
	}
	if err != nil && !feeIsNotEnough(err) {
		return types.Value{}, err
	}

	result := hi
	if !args.Flags.GetBit(types.MessageFlagInternal) {
		// Heuristic price for external message verification for the wallet.
		const externalVerificationGas = types.Gas(10_000)
		result = result.Add(externalVerificationGas.ToValue(res.GasPrice))
	}
	return refineResult(result), nil
}
