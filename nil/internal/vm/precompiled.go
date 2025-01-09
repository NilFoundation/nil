// Copyright 2014 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package vm

import (
	"errors"
	"fmt"
	"math"
	"math/big"
	"reflect"
	"slices"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/internal/abi"
	"github.com/NilFoundation/nil/nil/internal/config"
	"github.com/NilFoundation/nil/nil/internal/contracts"
	"github.com/NilFoundation/nil/nil/internal/tracing"
	"github.com/NilFoundation/nil/nil/internal/types"
	eth_common "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"
	"github.com/rs/zerolog/log"
)

func init() {
	check.PanicIfNot(eth_common.AddressLength == types.AddrSize)
}

func extractUintParam(arg any, methodName, paramName string) types.Value {
	valueBig, ok := arg.(*big.Int)
	check.PanicIfNotf(ok, "%s failed: `%s` argument is not big.Int", methodName, paramName)
	value, overflow := types.NewValueFromBig(valueBig)
	check.PanicIfNotf(!overflow, "%s failed: unexpected overflow in `%s`", methodName, paramName)
	return value
}

// PrecompiledContract is the basic interface for native Go contracts. The implementation
// requires a deterministic gas count based on the input size of the Run method of the
// contract.
type PrecompiledContract interface {
	// RequiredPrice calculates the contract gas use
	RequiredGas(input []byte, state StateDBReadOnly) (uint64, error)
}

type ReadOnlyPrecompiledContract interface {
	PrecompiledContract
	// Run runs the precompiled contract
	Run(state StateDBReadOnly, input []byte, value *uint256.Int, caller ContractRef) ([]byte, error)
}

type ReadWritePrecompiledContract interface {
	PrecompiledContract
	// Run runs the precompiled contract without state modifications
	Run(state StateDB, input []byte, value *uint256.Int, caller ContractRef) ([]byte, error)
}

type EvmAccessedPrecompiledContract interface {
	PrecompiledContract
	// Run runs the precompiled contract
	Run(evm *EVM, input []byte, value *uint256.Int, caller ContractRef) ([]byte, error)
}

type SimplePrecompiledContract interface {
	// RequiredPrice calculates the contract gas use
	RequiredGas(input []byte) uint64

	// Run runs the precompiled contract
	Run(input []byte) ([]byte, error)
}

var (
	SendRawMessageAddress  = types.BytesToAddress([]byte{0xfc})
	AsyncCallAddress       = types.BytesToAddress([]byte{0xfd})
	VerifySignatureAddress = types.BytesToAddress([]byte{0xfe})
	CheckIsInternalAddress = types.BytesToAddress([]byte{0xff})
	ManageCurrencyAddress  = types.BytesToAddress([]byte{0xd0})
	CurrencyBalanceAddress = types.BytesToAddress([]byte{0xd1})
	SendTokensAddress      = types.BytesToAddress([]byte{0xd2})
	MessageTokensAddress   = types.BytesToAddress([]byte{0xd3})
	GetGasPriceAddress     = types.BytesToAddress([]byte{0xd4})
	PoseidonHashAddress    = types.BytesToAddress([]byte{0xd5})
	AwaitCallAddress       = types.BytesToAddress([]byte{0xd6})
	ConfigParamAddress     = types.BytesToAddress([]byte{0xd7})
	SendRequestAddress     = types.BytesToAddress([]byte{0xd8})
	CheckIsResponseAddress = types.BytesToAddress([]byte{0xd9})
	LogAddress             = types.BytesToAddress([]byte{0xda})
)

// PrecompiledContractsPrague contains the set of pre-compiled Ethereum
// contracts used in the Prague release.
var PrecompiledContractsPrague = map[types.Address]PrecompiledContract{
	types.BytesToAddress([]byte{0x01}): &simple{&ecrecover{}},
	types.BytesToAddress([]byte{0x02}): &simple{&sha256hash{}},
	types.BytesToAddress([]byte{0x03}): &simple{&ripemd160hash{}},
	types.BytesToAddress([]byte{0x04}): &simple{&dataCopy{}},
	types.BytesToAddress([]byte{0x05}): &simple{&bigModExp{eip2565: true}},
	types.BytesToAddress([]byte{0x06}): &simple{&bn256AddIstanbul{}},
	types.BytesToAddress([]byte{0x07}): &simple{&bn256ScalarMulIstanbul{}},
	types.BytesToAddress([]byte{0x08}): &simple{&bn256PairingIstanbul{}},
	types.BytesToAddress([]byte{0x09}): &simple{&blake2F{}},
	types.BytesToAddress([]byte{0x0a}): &simple{&kzgPointEvaluation{}},
	types.BytesToAddress([]byte{0x0b}): &simple{&bls12381G1Add{}},
	types.BytesToAddress([]byte{0x0c}): &simple{&bls12381G1Mul{}},
	types.BytesToAddress([]byte{0x0d}): &simple{&bls12381G1MultiExp{}},
	types.BytesToAddress([]byte{0x0e}): &simple{&bls12381G2Add{}},
	types.BytesToAddress([]byte{0x0f}): &simple{&bls12381G2Mul{}},
	types.BytesToAddress([]byte{0x10}): &simple{&bls12381G2MultiExp{}},
	types.BytesToAddress([]byte{0x11}): &simple{&bls12381Pairing{}},
	types.BytesToAddress([]byte{0x12}): &simple{&bls12381MapG1{}},
	types.BytesToAddress([]byte{0x13}): &simple{&bls12381MapG2{}},

	// NilFoundation precompiled contracts
	SendRawMessageAddress:  &sendRawMessage{},
	AsyncCallAddress:       &asyncCall{},
	VerifySignatureAddress: &simple{&verifySignature{}},
	CheckIsInternalAddress: &checkIsInternal{},
	ManageCurrencyAddress:  &manageCurrency{},
	CurrencyBalanceAddress: &currencyBalance{},
	SendTokensAddress:      &sendCurrencySync{},
	MessageTokensAddress:   &getMessageTokens{},
	GetGasPriceAddress:     &getGasPrice{},
	PoseidonHashAddress:    &poseidonHash{},
	AwaitCallAddress:       &awaitCall{},
	ConfigParamAddress:     &configParam{},
	SendRequestAddress:     &sendRequest{},
	CheckIsResponseAddress: &checkIsResponse{},
	LogAddress:             &emitLog{},
}

// RunPrecompiledContract runs and evaluates the output of a precompiled contract.
// It returns
// - the returned bytes,
// - the _remaining_ gas,
// - any error that occurred
func RunPrecompiledContract(p PrecompiledContract, evm *EVM, input []byte, suppliedGas uint64,
	logger *tracing.Hooks, value *uint256.Int, caller ContractRef, readOnly bool,
) (ret []byte, remainingGas uint64, err error) {
	gasCost, err := p.RequiredGas(input, StateDBReadOnly(evm.StateDB))
	if err != nil {
		return nil, 0, err
	}
	if suppliedGas < gasCost {
		return nil, 0, fmt.Errorf("%w: %d < %d", ErrOutOfGas, suppliedGas, gasCost)
	}
	if logger != nil && logger.OnGasChange != nil {
		logger.OnGasChange(suppliedGas, suppliedGas-gasCost, tracing.GasChangeCallPrecompiledContract)
	}
	suppliedGas -= gasCost
	switch p := p.(type) {
	case ReadOnlyPrecompiledContract:
		ret, err = p.Run(StateDBReadOnly(evm.StateDB), input, value, caller)
	case ReadWritePrecompiledContract:
		if readOnly {
			err = ErrWriteProtection
		} else {
			ret, err = p.Run(evm.StateDB, input, value, caller)
		}
	case EvmAccessedPrecompiledContract:
		ret, err = p.Run(evm, input, value, caller)
	default:
		err = ErrUnexpectedPrecompileType
	}
	return ret, suppliedGas, err
}

type simple struct {
	contract SimplePrecompiledContract
}

var _ PrecompiledContract = (*simple)(nil)

func (a *simple) RequiredGas(input []byte, state StateDBReadOnly) (uint64, error) {
	return a.contract.RequiredGas(input), nil
}

func (a *simple) Run(_ StateDBReadOnly /* state */, input []byte, _ *uint256.Int /* value */, _ ContractRef /* caller */) ([]byte, error) {
	return a.contract.Run(input)
}

type sendRawMessage struct{}

var _ ReadWritePrecompiledContract = (*sendRawMessage)(nil)

const (
	// TODO: Make this dynamically calculated based on the network conditions and current shard gas price
	ForwardFee                   uint64    = 1_000
	ExtraForwardFeeStep          uint64    = 100
	MinGasReserveForAsyncRequest types.Gas = 50_000
)

func (c *sendRawMessage) RequiredGas([]byte, StateDBReadOnly) (uint64, error) {
	return ForwardFee, nil
}

func extractDstAddress(input []byte, methodName string, argNum int) (types.Address, error) {
	if len(input) < 4 {
		return types.EmptyAddress, types.NewVmError(types.ErrorPrecompileTooShortCallData)
	}

	// Unpack arguments, skipping the first 4 bytes (function selector)
	args, err := getPrecompiledMethod(methodName).Inputs.Unpack(input[4:])
	if err != nil {
		return types.EmptyAddress, types.NewVmError(types.ErrorAbiUnpackFailed)
	}
	if len(args) <= argNum {
		return types.EmptyAddress, types.NewVmError(types.ErrorPrecompileWrongNumberOfArguments)
	}

	// Get `dst` argument
	dst, ok := args[argNum].(types.Address)
	check.PanicIfNotf(ok, "dst argument is not an address")

	return dst, nil
}

func setRefundTo(refundTo *types.Address, msg *types.Message) {
	check.PanicIfNotf(msg != nil, "message is nil")

	if *refundTo == types.EmptyAddress {
		if msg.RefundTo == types.EmptyAddress {
			*refundTo = msg.From
		} else {
			*refundTo = msg.RefundTo
		}
	}
	if *refundTo == types.EmptyAddress {
		log.Logger.Warn().Msg("refund address is empty")
	}
}

func setBounceTo(bounceTo *types.Address, msg *types.Message) {
	if msg == nil {
		return
	}
	if *bounceTo == types.EmptyAddress {
		if msg.BounceTo == types.EmptyAddress {
			*bounceTo = msg.From
		} else {
			*bounceTo = msg.BounceTo
		}
	}
	if *bounceTo == types.EmptyAddress {
		log.Logger.Warn().Msg("bounce address is empty")
	}
}

func withdrawFunds(state StateDB, addr types.Address, value types.Value) error {
	if value.IsZero() {
		return nil
	}
	balance, err := state.GetBalance(addr)
	if err != nil {
		return err
	}
	if balance.Cmp(value) < 0 {
		log.Logger.Error().Msgf("withdrawFunds failed: insufficient balance on address %v, expected at least %v, got %v", addr, value, balance)
		return ErrInsufficientBalance
	}
	return state.SubBalance(addr, value, tracing.BalanceDecreasePrecompile)
}

func getPrecompiledMethod(methodName string) abi.Method {
	a, err := contracts.GetAbi(contracts.NamePrecompile)
	check.PanicIfErr(err)
	method, ok := a.Methods[methodName]
	check.PanicIfNotf(ok, "method %s not found", methodName)
	return method
}

// getBytesArgCopy returns a copy of the byte slice argument.
// It is needed because `abi.Unpack` unpack []byte arguments as a slice pointing inside the input calldata.
func getBytesArgCopy(arg any, methodName, paramName string) []byte {
	bytes, ok := arg.([]byte)
	check.PanicIfNotf(ok, "%s failed: `%s` is not a byte slice", methodName, paramName)
	return slices.Clone(bytes)
}

func (c *sendRawMessage) Run(state StateDB, input []byte, value *uint256.Int, caller ContractRef) ([]byte, error) {
	payload := new(types.InternalMessagePayload)
	if err := payload.UnmarshalSSZ(input); err != nil {
		return nil, types.NewWrapError(types.ErrorInvalidMessageInputUnmarshalFailed, err)
	}

	if payload.To.ShardId().IsMainShard() {
		return nil, ErrMessageToMainShard
	}

	if err := withdrawFunds(state, caller.Address(), payload.Value); err != nil {
		return []byte("sendRawMessage: withdraw value failed"), err
	}

	if payload.ForwardKind == types.ForwardKindNone {
		if err := withdrawFunds(state, caller.Address(), payload.FeeCredit); err != nil {
			return []byte("sendRawMessage: withdraw FeeCredit failed"), err
		}
	}

	// TODO: We should consider non-refundable messages
	setRefundTo(&payload.RefundTo, state.GetInMessage())
	setBounceTo(&payload.BounceTo, state.GetInMessage())

	_, err := state.AddOutMessage(caller.Address(), payload)

	return nil, err
}

var gasScale = types.DefaultGasPrice.Div(types.Value100)

// GetExtraGasForOutboundMessage returns the extra gas required for sending a message to a shard according to its gas
// price. If the gas price is higher than the default gas price, the extra gas will be higher.
func GetExtraGasForOutboundMessage(state StateDBReadOnly, shardId types.ShardId) uint64 {
	gasPrice, err := state.GetGasPrice(shardId)
	if err != nil {
		log.Logger.Error().Msgf("GetExtraGasForOutboundMessage failed to get gas price: %s", err)
		return 0
	}

	if gasPrice.Cmp(types.DefaultGasPrice) > 0 {
		diff := gasPrice.Sub(types.DefaultGasPrice)
		extraGas := diff.Div(gasScale)
		return ExtraForwardFeeStep * extraGas.Uint64()
	}

	return uint64(0)
}

type asyncCall struct{}

var _ ReadWritePrecompiledContract = (*asyncCall)(nil)

func (c *asyncCall) RequiredGas(input []byte, state StateDBReadOnly) (uint64, error) {
	dst, err := extractDstAddress(input, "precompileAsyncCall", 3)
	if err != nil {
		return 0, err
	}

	extraGas := GetExtraGasForOutboundMessage(state, dst.ShardId())

	return ForwardFee + extraGas, nil
}

func extractCurrencies(arg any) ([]types.CurrencyBalance, error) {
	slice := reflect.ValueOf(arg)
	currencies := make([]types.CurrencyBalance, slice.Len())
	if slice.Len() >= types.MessageMaxCurrencySize {
		return nil, types.NewVmError(types.ErrorPrecompileCurrencyArrayIsTooBig)
	}
	for i := range slice.Len() {
		elem := slice.Index(i)
		currencyId, ok := elem.FieldByIndex([]int{0}).Interface().(types.Address)
		if !ok {
			return nil, errors.New("currencyId is not an Address type")
		}
		currencies[i].Currency = types.CurrencyId(currencyId)

		balanceBig, ok := elem.FieldByIndex([]int{1}).Interface().(*big.Int)
		if !ok {
			return nil, errors.New("balance is not a big.Int")
		}
		currencies[i].Balance = types.NewValueFromBigMust(balanceBig)
	}
	return currencies, nil
}

func (c *asyncCall) Run(state StateDB, input []byte, value *uint256.Int, caller ContractRef) (res []byte, err error) {
	if len(input) < 4 {
		return nil, types.NewVmError(types.ErrorPrecompileTooShortCallData)
	}

	// Unpack arguments, skipping the first 4 bytes (function selector)
	args, err := getPrecompiledMethod("precompileAsyncCall").Inputs.Unpack(input[4:])
	if err != nil {
		return nil, types.NewVmVerboseError(types.ErrorAbiUnpackFailed, err.Error())
	}
	if len(args) != 8 {
		return nil, types.NewVmError(types.ErrorPrecompileWrongNumberOfArguments)
	}

	// Get `isDeploy` argument
	deploy, ok := args[0].(bool)
	check.PanicIfNotf(ok, "isDeploy is not a bool: %v", args[0])

	// Get `forwardKind` argument
	forwardKind, ok := args[1].(uint8)
	check.PanicIfNotf(ok, "asyncCall failed: forwardKind argument is not an uint8")

	// Get `dst` argument
	dst, ok := args[2].(types.Address)
	check.PanicIfNotf(ok, "asyncCall failed: dst argument is not an address")

	// Get `refundTo` argument
	refundTo, ok := args[3].(types.Address)
	check.PanicIfNotf(ok, "asyncCall failed: refundTo argument is not an address")

	// Get `bounceTo` argument
	bounceTo, ok := args[4].(types.Address)
	check.PanicIfNotf(ok, "asyncCall failed: bounceTo argument is not an address")

	// Get `feeCredit` argument
	feeCredit := extractUintParam(args[5], "asyncCall", "feeCredit")

	// Get `currencies` argument, which is a slice of `CurrencyBalance`
	currencies, err := extractCurrencies(args[6])
	if err != nil {
		log.Logger.Error().Err(err).Msgf("failed to extract currencies from %T", args[6])
		if types.IsVmError(err) {
			return nil, err
		}
		return nil, types.NewVmVerboseError(types.ErrorPrecompileInvalidCurrencyArray, err.Error())
	}

	// Get `input` argument
	input = getBytesArgCopy(args[7], "asyncCall", "input")

	var kind types.MessageKind
	if deploy {
		if len(currencies) != 0 {
			return nil, types.NewVmVerboseError(types.ErrorAsyncDeployMustNotHaveCurrency, err.Error())
		}
		kind = types.DeployMessageKind
	} else {
		kind = types.ExecutionMessageKind
	}

	if dst.ShardId().IsMainShard() {
		return []byte("asyncCall failed: attempt to send message to main shard"), ErrMessageToMainShard
	}

	if forwardKind == types.ForwardKindNone {
		if err := withdrawFunds(state, caller.Address(), feeCredit); err != nil {
			return []byte("asyncCall failed: withdrawFunds failed"), err
		}
	}

	if err := withdrawFunds(state, caller.Address(), types.NewValue(value)); err != nil {
		return []byte("asyncCall failed: withdrawFunds failed"), err
	}

	// TODO: We should consider non-refundable messages
	setRefundTo(&refundTo, state.GetInMessage())
	setBounceTo(&bounceTo, state.GetInMessage())

	// Internal is required for the message
	payload := types.InternalMessagePayload{
		Kind:        kind,
		FeeCredit:   feeCredit,
		ForwardKind: types.ForwardKind(forwardKind),
		Value:       types.NewValue(value),
		Currency:    currencies,
		To:          dst,
		RefundTo:    refundTo,
		BounceTo:    bounceTo,
		Data:        input,
	}
	res = make([]byte, 32)
	res[31] = 1

	_, err = state.AddOutMessage(caller.Address(), &payload)

	return res, err
}

func estimateGasForAsyncRequest(input []byte, precompile string, argnum, argtotal int) uint64 {
	if len(input) < 4 {
		return 0
	}

	// when running `awaitCall` / `sendRequest` the caller specifies exact amount of gas they want to reserve
	// later this gas will be used for processing of response for particular request
	method := getPrecompiledMethod(precompile)

	// particular const value will be adjusted later
	baseFee := 4000 + ForwardFee

	// Unpack arguments, skipping the first 4 bytes (function selector)
	args, err := method.Inputs.Unpack(input[4:])
	// We don't need to tackle somehow any unpacking errors, cause running the contract with
	// wrong argument will fail anyway (inside `Run` function)
	if err != nil || len(args) != argtotal {
		return baseFee
	}

	responseProcessingGas := extractUintParam(args[argnum], precompile, "responseProcessingGas")
	return baseFee + responseProcessingGas.Uint64()
}

type awaitCall struct{}

var _ EvmAccessedPrecompiledContract = (*awaitCall)(nil)

func (c *awaitCall) RequiredGas(input []byte, state StateDBReadOnly) (uint64, error) {
	dst, err := extractDstAddress(input, "precompileAwaitCall", 0)
	if err != nil {
		return math.MaxUint64, err
	}
	extraGas := GetExtraGasForOutboundMessage(state, dst.ShardId())

	return extraGas + estimateGasForAsyncRequest(input, "precompileAwaitCall", 1, 3), nil
}

func (a *awaitCall) Run(evm *EVM, input []byte, value *uint256.Int, caller ContractRef) ([]byte, error) {
	if len(input) < 4 {
		return nil, types.NewVmError(types.ErrorPrecompileTooShortCallData)
	}

	state := evm.StateDB

	// Only top level functions are allowed to use awaitCall
	if evm.GetDepth() > 1 {
		return nil, types.NewVmError(types.ErrorAwaitCallCalledFromNotTopLevel)
	}

	method := getPrecompiledMethod("precompileAwaitCall")

	// Unpack arguments, skipping the first 4 bytes (function selector)
	args, err := method.Inputs.Unpack(input[4:])
	if err != nil {
		return nil, types.NewVmVerboseError(types.ErrorAbiUnpackFailed, err.Error())
	}
	if len(args) != 3 {
		return nil, types.NewVmError(types.ErrorPrecompileWrongNumberOfArguments)
	}

	// Get `dst` argument
	dst, ok := args[0].(types.Address)
	check.PanicIfNotf(ok, "awaitCall failed: dst argument is not an address")

	// Get `responseProcessingGas` argument
	responseProcessingGas := types.Gas(extractUintParam(args[1], "awaitCall", "responseProcessingGas").Uint64())
	if responseProcessingGas < MinGasReserveForAsyncRequest {
		log.Logger.Error().Msgf("awaitCall failed: responseProcessingGas is too low (%d)", responseProcessingGas)
		return nil, types.NewVmError(types.ErrorAwaitCallTooLowResponseProcessingGas)
	}

	// Get `callData` argument
	callData := getBytesArgCopy(args[2], "awaitCall", "callData")

	// Internal is required for the message
	payload := types.InternalMessagePayload{
		Kind:        types.ExecutionMessageKind,
		FeeCredit:   types.NewZeroValue(),
		ForwardKind: types.ForwardKindRemaining,
		Value:       types.NewValue(value),
		Currency:    nil,
		To:          dst,
		BounceTo:    state.GetInMessage().To,
		Data:        callData,
	}

	setRefundTo(&payload.RefundTo, state.GetInMessage())

	if _, err = state.AddOutRequestMessage(caller.Address(), &payload, responseProcessingGas, true); err != nil {
		log.Logger.Error().Msgf("AddOutRequestMessage failed: %s", err)
		return nil, types.NewVmVerboseError(types.ErrorPrecompileStateDbReturnedError, err.Error())
	}

	return nil, nil
}

type sendRequest struct{}

var _ ReadWritePrecompiledContract = (*sendRequest)(nil)

func (c *sendRequest) RequiredGas(input []byte, state StateDBReadOnly) (uint64, error) {
	dst, err := extractDstAddress(input, "precompileSendRequest", 0)
	if err != nil {
		return math.MaxUint64, err
	}
	extraGas := GetExtraGasForOutboundMessage(state, dst.ShardId())

	return extraGas + estimateGasForAsyncRequest(input, "precompileSendRequest", 2, 5), nil
}

func (a *sendRequest) Run(state StateDB, input []byte, value *uint256.Int, caller ContractRef) ([]byte, error) {
	if len(input) < 4 {
		return nil, types.NewVmError(types.ErrorPrecompileTooShortCallData)
	}

	method := getPrecompiledMethod("precompileSendRequest")

	// Unpack arguments, skipping the first 4 bytes (function selector)
	args, err := method.Inputs.Unpack(input[4:])
	if err != nil {
		return nil, types.NewVmVerboseError(types.ErrorAbiUnpackFailed, err.Error())
	}
	if len(args) != 5 {
		return nil, types.NewVmError(types.ErrorPrecompileWrongNumberOfArguments)
	}

	// Get `dst` argument
	dst, ok := args[0].(types.Address)
	check.PanicIfNotf(ok, "sendRequest failed: dst argument is not an address")

	// Get `currencies` argument, which is a slice of `CurrencyBalance`
	currencies, err := extractCurrencies(args[1])
	if err != nil {
		log.Logger.Error().Err(err).Msg("currencies is not a slice of CurrencyBalance")
		return nil, types.NewVmVerboseError(types.ErrorPrecompileInvalidCurrencyArray, err.Error())
	}

	// Get `responseProcessingGas` argument
	responseProcessingGas := types.Gas(extractUintParam(args[2], "sendRequest", "responseProcessingGas").Uint64())
	if responseProcessingGas < MinGasReserveForAsyncRequest {
		log.Logger.Error().Msgf("sendRequest failed: responseProcessingGas is too low (%d)", responseProcessingGas)
		return nil, types.NewVmError(types.ErrorAwaitCallTooLowResponseProcessingGas)
	}

	// Get `context` argument
	context := getBytesArgCopy(args[3], "sendRequest", "context")

	// Get `callData` argument
	callData := getBytesArgCopy(args[4], "sendRequest", "callData")

	if err := withdrawFunds(state, caller.Address(), types.NewValue(value)); err != nil {
		return []byte("sendRequest failed: withdrawFunds failed"), err
	}

	// Internal is required for the message
	payload := types.InternalMessagePayload{
		Kind:           types.ExecutionMessageKind,
		FeeCredit:      types.NewZeroValue(),
		ForwardKind:    types.ForwardKindRemaining,
		Value:          types.NewValue(value),
		Currency:       currencies,
		To:             dst,
		BounceTo:       state.GetInMessage().To,
		Data:           callData,
		RequestContext: context,
	}

	setRefundTo(&payload.RefundTo, state.GetInMessage())

	if _, err = state.AddOutRequestMessage(caller.Address(), &payload, responseProcessingGas, false); err != nil {
		log.Logger.Error().Msgf("AddOutRequestMessage failed: %s", err)
		return nil, types.NewVmVerboseError(types.ErrorPrecompileStateDbReturnedError, err.Error())
	}

	res := make([]byte, 32)
	res[31] = 1

	return res, nil
}

type verifySignature struct{}

var _ SimplePrecompiledContract = (*verifySignature)(nil)

func (c *verifySignature) RequiredGas([]byte) uint64 {
	return 5000
}

func (a *verifySignature) Run(input []byte) ([]byte, error) {
	args := VerifySignatureArgs()
	values, err := args.Unpack(input)
	if err != nil || len(values) != 3 {
		return common.EmptyHash[:], nil //nolint:nilerr
	}
	// there's probably a better way to do this
	pubkey, ok1 := values[0].([]byte)
	hash, ok2 := values[1].(*big.Int)
	sig, ok3 := values[2].([]byte)
	if !(ok1 && ok2 && ok3 && len(sig) == 65) {
		return common.EmptyHash[:], nil
	}
	result := crypto.VerifySignature(pubkey, common.BigToHash(hash).Bytes(), sig[:64])
	if result {
		return common.LeftPadBytes([]byte{1}, 32), nil
	}
	return common.EmptyHash[:], nil
}

func VerifySignatureArgs() abi.Arguments {
	// arguments: bytes pubkey, uint256 hash, bytes signature
	// returns: bool signatureValid
	uint256Ty, _ := abi.NewType("uint256", "", nil)
	bytesTy, _ := abi.NewType("bytes", "", nil)
	args := abi.Arguments{
		abi.Argument{Name: "pubkey", Type: bytesTy},
		abi.Argument{Name: "hash", Type: uint256Ty},
		abi.Argument{Name: "signature", Type: bytesTy},
	}
	return args
}

type checkIsInternal struct{}

var _ ReadOnlyPrecompiledContract = (*checkIsInternal)(nil)

func (c *checkIsInternal) RequiredGas([]byte, StateDBReadOnly) (uint64, error) {
	return 10, nil
}

func (a *checkIsInternal) Run(state StateDBReadOnly, input []byte, value *uint256.Int, caller ContractRef) ([]byte, error) {
	res := make([]byte, 32)

	if state.IsInternalMessage() {
		res[31] = 1
	}

	return res, nil
}

type checkIsResponse struct{}

var _ ReadOnlyPrecompiledContract = (*checkIsResponse)(nil)

func (c *checkIsResponse) RequiredGas([]byte, StateDBReadOnly) (uint64, error) {
	return 10, nil
}

func (a *checkIsResponse) Run(state StateDBReadOnly, input []byte, value *uint256.Int, caller ContractRef) ([]byte, error) {
	if !state.GetMessageFlags().IsResponse() {
		return nil, types.NewVmError(types.ErrorOnlyResponseCheckFailed)
	}

	res := make([]byte, 32)
	res[31] = 1

	return res, nil
}

type manageCurrency struct{}

var _ ReadWritePrecompiledContract = (*manageCurrency)(nil)

func (c *manageCurrency) RequiredGas([]byte, StateDBReadOnly) (uint64, error) {
	return 10, nil
}

func (c *manageCurrency) Run(state StateDB, input []byte, value *uint256.Int, caller ContractRef) ([]byte, error) {
	if len(input) < 4 {
		return nil, types.NewVmError(types.ErrorPrecompileTooShortCallData)
	}

	res := make([]byte, 32)

	args, err := getPrecompiledMethod("precompileManageCurrency").Inputs.Unpack(input[4:])
	if err != nil {
		return nil, types.NewVmVerboseError(types.ErrorAbiUnpackFailed, err.Error())
	}
	if len(args) != 2 {
		return nil, types.NewVmError(types.ErrorPrecompileWrongNumberOfArguments)
	}

	amountBig, ok := args[0].(*big.Int)
	check.PanicIfNotf(ok, "manageCurrency failed: `amountBig` is not a big.Int: %v", args[0])
	amount := types.NewValueFromBigMust(amountBig)

	mint, ok := args[1].(bool)
	check.PanicIfNotf(ok, "manageCurrency failed: `mint` is not a bool: %v", args[1])

	currencyId := types.CurrencyId(caller.Address())

	action := state.AddCurrency
	if !mint {
		action = state.SubCurrency
	}

	if err = action(caller.Address(), currencyId, amount); err != nil {
		actionName := "AddCurrency"
		if !mint {
			actionName = "SubCurrency"
		}
		return nil, types.NewVmVerboseError(types.ErrorPrecompileWrongNumberOfArguments, fmt.Sprintf("%s failed: %v", actionName, err))
	}

	// Set return data to boolean `true` value
	res[31] = 1

	return res, nil
}

type currencyBalance struct{}

var _ ReadOnlyPrecompiledContract = (*currencyBalance)(nil)

func (c *currencyBalance) RequiredGas([]byte, StateDBReadOnly) (uint64, error) {
	return 10, nil
}

func (a *currencyBalance) Run(state StateDBReadOnly, input []byte, value *uint256.Int, caller ContractRef) ([]byte, error) {
	if len(input) < 4 {
		return nil, types.NewVmError(types.ErrorPrecompileTooShortCallData)
	}

	res := make([]byte, 32)

	// Unpack arguments, skipping the first 4 bytes (function selector)
	args, err := getPrecompiledMethod("precompileGetCurrencyBalance").Inputs.Unpack(input[4:])
	if err != nil {
		return nil, types.NewVmVerboseError(types.ErrorAbiUnpackFailed, err.Error())
	}
	if len(args) != 2 {
		return nil, types.NewVmError(types.ErrorPrecompileWrongNumberOfArguments)
	}

	// Get `id` argument
	currencyId, ok := args[0].(types.Address)
	check.PanicIfNotf(ok, "currencyBalance failed: currencyId is not an Address: %v", args[0])

	// Get `addr` argument
	addr, ok := args[1].(types.Address)
	check.PanicIfNotf(ok, "currencyBalance failed: addr argument is not an address")

	if addr == types.EmptyAddress {
		addr = caller.Address()
	} else if addr.ShardId() != caller.Address().ShardId() {
		return nil, types.NewVmVerboseError(types.ErrorCrossShardMessage, "currencyBalance")
	}

	currencies := state.GetCurrencies(addr)
	r, ok := currencies[types.CurrencyId(currencyId)]
	if ok {
		b := r.Bytes32()
		return b[:], nil
	}

	return res, nil
}

type sendCurrencySync struct{}

var _ ReadWritePrecompiledContract = (*sendCurrencySync)(nil)

func (c *sendCurrencySync) RequiredGas([]byte, StateDBReadOnly) (uint64, error) {
	return 10, nil
}

func (c *sendCurrencySync) Run(state StateDB, input []byte, value *uint256.Int, caller ContractRef) ([]byte, error) {
	if len(input) < 4 {
		return nil, types.NewVmError(types.ErrorPrecompileTooShortCallData)
	}

	// Unpack arguments, skipping the first 4 bytes (function selector)
	args, err := getPrecompiledMethod("precompileSendTokens").Inputs.Unpack(input[4:])
	if err != nil {
		return nil, types.NewVmVerboseError(types.ErrorAbiUnpackFailed, err.Error())
	}
	if len(args) != 2 {
		return nil, types.NewVmError(types.ErrorPrecompileWrongNumberOfArguments)
	}

	// Get destination address
	addr, ok := args[0].(types.Address)
	check.PanicIfNotf(ok, "sendCurrencySync failed: addr argument is not an address")

	if caller.Address().ShardId() != addr.ShardId() {
		return nil, fmt.Errorf("sendCurrencySync: %w: %s -> %s",
			ErrCrossShardMessage, caller.Address().ShardId(), addr.ShardId())
	}

	// Get currencies
	currencies, err := extractCurrencies(args[1])
	if err != nil {
		return nil, types.NewVmVerboseError(types.ErrorPrecompileInvalidCurrencyArray, "sendCurrencySync")
	}

	state.SetCurrencyTransfer(currencies)

	res := make([]byte, 32)
	res[31] = 1

	return res, nil
}

type getMessageTokens struct{}

var _ ReadOnlyPrecompiledContract = (*getMessageTokens)(nil)

func (c *getMessageTokens) RequiredGas([]byte, StateDBReadOnly) (uint64, error) {
	return 10, nil
}

func (c *getMessageTokens) Run(state StateDBReadOnly, input []byte, value *uint256.Int, caller ContractRef) ([]byte, error) {
	callerCurrencies := caller.Currency()
	res, err := getPrecompiledMethod("precompileGetMessageTokens").Outputs.Pack(callerCurrencies)
	if err != nil {
		return nil, types.NewVmVerboseError(types.ErrorAbiPackFailed, err.Error())
	}

	return res, nil
}

type getGasPrice struct{}

var _ ReadOnlyPrecompiledContract = (*getGasPrice)(nil)

func (c *getGasPrice) RequiredGas([]byte, StateDBReadOnly) (uint64, error) {
	return 10, nil
}

func (c *getGasPrice) Run(state StateDBReadOnly, input []byte, value *uint256.Int, caller ContractRef) ([]byte, error) {
	if len(input) < 4 {
		return nil, types.NewVmError(types.ErrorPrecompileTooShortCallData)
	}

	method := getPrecompiledMethod("precompileGetGasPrice")

	args, err := method.Inputs.Unpack(input[4:])
	if err != nil {
		return nil, types.NewVmVerboseError(types.ErrorAbiUnpackFailed, err.Error())
	}
	if len(args) != 1 {
		return nil, types.NewVmError(types.ErrorPrecompileWrongNumberOfArguments)
	}

	// Get `shardId` argument
	shardId, ok := args[0].(*big.Int)
	check.PanicIfNotf(ok, "getGasPrice failed: shardId is not a big.Int: %v", args[0])
	if !shardId.IsUint64() {
		return nil, types.NewVmVerboseError(types.ErrorShardIdIsTooBig, "getGasPrice")
	}

	gasPrice, err := state.GetGasPrice(types.ShardId(shardId.Uint64()))
	if err != nil {
		return nil, types.NewVmVerboseError(types.ErrorPrecompileStateDbReturnedError, err.Error())
	}

	res, err := method.Outputs.Pack(gasPrice.ToBig())
	if err != nil {
		return nil, types.NewVmVerboseError(types.ErrorAbiPackFailed, err.Error())
	}

	return res, nil
}

type poseidonHash struct{}

var _ ReadOnlyPrecompiledContract = (*poseidonHash)(nil)

func (c *poseidonHash) RequiredGas([]byte, StateDBReadOnly) (uint64, error) {
	return 10, nil
}

func (c *poseidonHash) Run(state StateDBReadOnly, input []byte, value *uint256.Int, caller ContractRef) ([]byte, error) {
	if len(input) < 4 {
		return nil, types.NewVmError(types.ErrorPrecompileTooShortCallData)
	}

	method := getPrecompiledMethod("precompileGetPoseidonHash")

	args, err := method.Inputs.Unpack(input[4:])
	if err != nil {
		return nil, types.NewVmVerboseError(types.ErrorAbiUnpackFailed, err.Error())
	}
	if len(args) != 1 {
		return nil, types.NewVmError(types.ErrorPrecompileWrongNumberOfArguments)
	}

	// Get `data` argument
	data, ok := args[0].([]byte)
	check.PanicIfNotf(ok, "poseidonHash failed: data is not a bytes: %v", args[0])

	return common.PoseidonHash(data).Bytes(), nil
}

type configParam struct{}

var _ ReadWritePrecompiledContract = (*configParam)(nil)

func (c *configParam) RequiredGas([]byte, StateDBReadOnly) (uint64, error) {
	return 10, nil
}

func (c *configParam) Run(state StateDB, input []byte, value *uint256.Int, caller ContractRef) ([]byte, error) {
	if len(input) < 4 {
		return nil, types.NewVmError(types.ErrorPrecompileTooShortCallData)
	}

	method := getPrecompiledMethod("precompileConfigParam")

	args, err := method.Inputs.Unpack(input[4:])
	if err != nil {
		return nil, err
	}
	if len(args) != 3 {
		return nil, types.NewVmError(types.ErrorPrecompileWrongNumberOfArguments)
	}

	// Get `isSet` argument
	isSet, ok := args[0].(bool)
	check.PanicIfNotf(ok, "configParam failed: isSet is not a bool")

	// Get `name` argument
	name, ok := args[1].(string)
	check.PanicIfNotf(ok, "configParam failed: name is not a string")

	cfgAccessor := state.GetConfigAccessor()

	if isSet {
		// Get `data` argument
		data, ok := args[2].([]byte)
		check.PanicIfNotf(ok, "configParam failed: data is not a []byte")

		params, err := config.UnpackSolidity(name, data)
		if err != nil {
			return nil, types.NewVmVerboseError(types.ErrorAbiUnpackFailed, err.Error())
		}

		if !state.GetShardID().IsMainShard() {
			return nil, types.NewVmError(types.ErrorOnlyMainShardContractsCanChangeConfig)
		}

		if err = config.SetParam(cfgAccessor, name, params); err != nil {
			return nil, types.NewVmVerboseError(types.ErrorPrecompileConfigSetParamFailed, err.Error())
		}

		return method.Outputs.Pack([]byte{})
	}
	params, err := config.GetParam(cfgAccessor, name)
	if err != nil {
		return nil, types.NewVmVerboseError(types.ErrorPrecompileConfigGetParamFailed, err.Error())
	}
	data, err := config.PackSolidity(name, params)
	if err != nil {
		return nil, types.NewVmVerboseError(types.ErrorAbiPackFailed, err.Error())
	}

	return method.Outputs.Pack(data)
}

type emitLog struct{}

var _ ReadWritePrecompiledContract = (*emitLog)(nil)

func (e *emitLog) RequiredGas([]byte, StateDBReadOnly) (uint64, error) {
	return 1000, nil
}

func (e *emitLog) Run(state StateDB, input []byte, value *uint256.Int, caller ContractRef) ([]byte, error) {
	if len(input) < 4 {
		return nil, types.NewVmError(types.ErrorPrecompileTooShortCallData)
	}

	method := getPrecompiledMethod("precompileLog")

	args, err := method.Inputs.Unpack(input[4:])
	if err != nil {
		return nil, err
	}
	if len(args) != 2 {
		return nil, types.NewVmError(types.ErrorPrecompileWrongNumberOfArguments)
	}

	// Get `message` argument
	message, ok := args[0].(string)
	if !ok {
		return nil, types.NewVmError(types.ErrorAbiUnpackFailed)
	}

	// Get `data` argument
	slice := reflect.ValueOf(args[1])
	data := make([]types.Uint256, slice.Len())
	for i := range slice.Len() {
		if v, ok := slice.Index(i).Interface().(*big.Int); ok {
			data[i].SetFromBig(v)
		} else {
			return nil, types.NewVmError(types.ErrorAbiUnpackFailed)
		}
	}

	debugLog, err := types.NewDebugLog([]byte(message), data)
	if err != nil {
		return nil, types.KeepOrWrapError(types.ErrorEmitDebugLogFailed, err)
	}
	if err = state.AddDebugLog(debugLog); err != nil {
		return nil, types.KeepOrWrapError(types.ErrorEmitDebugLogFailed, err)
	}

	res := make([]byte, 32)
	res[31] = 1

	return res, nil
}
