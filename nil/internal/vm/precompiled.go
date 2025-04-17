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
	"math/big"
	"reflect"
	"slices"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/abi"
	"github.com/NilFoundation/nil/nil/internal/config"
	"github.com/NilFoundation/nil/nil/internal/contracts"
	"github.com/NilFoundation/nil/nil/internal/tracing"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/internal/vm/console"
	eth_common "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"
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
	AsyncCallAddress         = types.BytesToAddress([]byte{0xfd})
	VerifySignatureAddress   = types.BytesToAddress([]byte{0xfe})
	CheckIsInternalAddress   = types.BytesToAddress([]byte{0xff})
	ManageTokenAddress       = types.BytesToAddress([]byte{0xd0})
	TokenBalanceAddress      = types.BytesToAddress([]byte{0xd1})
	SendTokensAddress        = types.BytesToAddress([]byte{0xd2})
	TransactionTokensAddress = types.BytesToAddress([]byte{0xd3})
	GetGasPriceAddress       = types.BytesToAddress([]byte{0xd4})
	ConfigParamAddress       = types.BytesToAddress([]byte{0xd7})
	CheckIsResponseAddress   = types.BytesToAddress([]byte{0xd9})
	LogAddress               = types.BytesToAddress([]byte{0xda})
	GovernanceAddress        = types.BytesToAddress([]byte{0xdb})
	ConsoleAddress           = types.HexToAddress("0x00000000000000000000000000000000000dEBa6")
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
	AsyncCallAddress:         &asyncCall{},
	VerifySignatureAddress:   &simple{&verifySignature{}},
	CheckIsInternalAddress:   &checkIsInternal{},
	ManageTokenAddress:       &manageToken{},
	TokenBalanceAddress:      &tokenBalance{},
	SendTokensAddress:        &sendTokenSync{},
	TransactionTokensAddress: &getTransactionTokens{},
	GetGasPriceAddress:       &getGasPrice{},
	ConfigParamAddress:       &configParam{},
	CheckIsResponseAddress:   &checkIsResponse{},
	LogAddress:               &emitLog{},
	GovernanceAddress:        &governance{},
	ConsoleAddress:           &consolePrecompile{},
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
		return nil, 0, types.NewVerboseError(types.ErrorOutOfGasForPrecompile,
			fmt.Sprintf("%d < %d", suppliedGas, gasCost))
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

var _ ReadOnlyPrecompiledContract = (*simple)(nil)

func (a *simple) RequiredGas(input []byte, state StateDBReadOnly) (uint64, error) {
	return a.contract.RequiredGas(input), nil
}

func (a *simple) Run(
	_ StateDBReadOnly, /* state */
	input []byte,
	_ *uint256.Int, /* value */
	_ ContractRef, /* caller */
) ([]byte, error) {
	return a.contract.Run(input)
}

const (
	// TODO: Make this dynamically calculated based on the network conditions and current shard gas price
	ForwardFee                   uint64    = 1_000
	ExtraForwardFeeStep          uint64    = 100
	MinGasReserveForAsyncRequest types.Gas = 50_000
)

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

func setRefundTo(refundTo *types.Address, txn *types.Transaction) {
	check.PanicIfNotf(txn != nil, "transaction is nil")

	if *refundTo == types.EmptyAddress {
		if txn.RefundTo == types.EmptyAddress {
			*refundTo = txn.From
		} else {
			*refundTo = txn.RefundTo
		}
	}
	if *refundTo == types.EmptyAddress {
		logging.GlobalLogger.Warn().Msg("refund address is empty")
	}
}

func setBounceTo(bounceTo *types.Address, txn *types.Transaction) {
	if txn == nil {
		return
	}
	if *bounceTo == types.EmptyAddress {
		if txn.BounceTo == types.EmptyAddress {
			*bounceTo = txn.From
		} else {
			*bounceTo = txn.BounceTo
		}
	}
	if *bounceTo == types.EmptyAddress {
		logging.GlobalLogger.Warn().Msg("bounce address is empty")
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
		logging.GlobalLogger.Error().Msgf(
			"withdrawFunds failed: insufficient balance on address %v, expected at least %v, got %v",
			addr, value, balance)
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

var gasScale = types.DefaultGasPrice.Div(types.Value100)

// GetExtraGasForOutboundTransaction returns the extra gas required for sending a transaction to a shard
// according to its gas price. If the gas price is higher than the default gas price, the extra gas will be higher.
func GetExtraGasForOutboundTransaction(state StateDBReadOnly, shardId types.ShardId) uint64 {
	gasPrice, err := state.GetGasPrice(shardId)
	if err != nil {
		logging.GlobalLogger.Error().Msgf("GetExtraGasForOutboundTransaction failed to get gas price: %s", err)
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

func estimateGasForAsyncRequest(input []byte, precompile string, argnum, argtotal int) uint64 {
	if len(input) < 4 {
		return 0
	}

	// when running `asyncCall` the caller specifies exact amount of gas they want to reserve
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

func (c *asyncCall) RequiredGas(input []byte, state StateDBReadOnly) (uint64, error) {
	dst, err := extractDstAddress(input, "precompileAsyncCall", 3)
	if err != nil {
		return 0, err
	}

	extraGas := GetExtraGasForOutboundTransaction(state, dst.ShardId())

	return extraGas + estimateGasForAsyncRequest(input, "precompileAsyncCall", 9, 10), nil
}

func extractTokens(arg any) ([]types.TokenBalance, error) {
	slice := reflect.ValueOf(arg)
	tokens := make([]types.TokenBalance, slice.Len())
	if slice.Len() >= types.TransactionMaxTokenSize {
		return nil, types.NewVmError(types.ErrorPrecompileTokenArrayIsTooBig)
	}
	for i := range slice.Len() {
		elem := slice.Index(i)
		tokenId, ok := elem.FieldByIndex([]int{0}).Interface().(types.Address)
		if !ok {
			return nil, errors.New("tokenId is not an Address type")
		}
		tokens[i].Token = types.TokenId(tokenId)

		balanceBig, ok := elem.FieldByIndex([]int{1}).Interface().(*big.Int)
		if !ok {
			return nil, errors.New("balance is not a big.Int")
		}
		tokens[i].Balance = types.NewValueFromBigMust(balanceBig)
	}
	return tokens, nil
}

func (c *asyncCall) Run(state StateDB, input []byte, value *uint256.Int, caller ContractRef) (res []byte, err error) {
	if len(input) < 4 {
		return nil, types.NewVmError(types.ErrorPrecompileTooShortCallData)
	}

	cfgAccessor := state.GetConfigAccessor()
	nShards, err := config.GetParamNShards(cfgAccessor)
	if err != nil {
		return nil, types.NewVmVerboseError(types.ErrorPrecompileConfigGetParamFailed, err.Error())
	}

	// Unpack arguments, skipping the first 4 bytes (function selector)
	args, err := getPrecompiledMethod("precompileAsyncCall").Inputs.Unpack(input[4:])
	if err != nil {
		return nil, types.NewVmVerboseError(types.ErrorAbiUnpackFailed, err.Error())
	}
	if len(args) != 10 {
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

	// Get `tokens` argument, which is a slice of `TokenBalance`
	tokens, err := extractTokens(args[6])
	if err != nil {
		logging.GlobalLogger.Error().Err(err).Msgf("failed to extract tokens from %T", args[6])
		if types.IsVmError(err) {
			return nil, err
		}
		return nil, types.NewVmVerboseError(types.ErrorPrecompileInvalidTokenArray, err.Error())
	}

	// Get `input` argument
	input = getBytesArgCopy(args[7], "asyncCall", "input")

	// Get `context` argument
	context := getBytesArgCopy(args[8], "asyncCall", "context")

	// Get `responseProcessingGas` argument
	responseProcessingGas := types.Gas(extractUintParam(args[9], "asyncCall", "responseProcessingGas").Uint64())
	if len(context) > 0 && responseProcessingGas < MinGasReserveForAsyncRequest {
		logging.GlobalLogger.Error().Msgf(
			"asyncCall failed: responseProcessingGas is too low (%d)", responseProcessingGas)
		return nil, types.NewVmError(types.ErrorTooLowResponseProcessingGas)
	}
	if len(context) == 0 && responseProcessingGas > 0 {
		return nil, types.NewVmError(types.ErrorResponseProcessingGasWithoutResponse)
	}
	if 0 < len(context) && len(context) < 4 {
		return nil, types.NewVmError(types.ErrorTooShortContextData)
	}

	var kind types.TransactionKind
	if deploy {
		if len(tokens) != 0 {
			return nil, types.NewVmError(types.ErrorAsyncDeployMustNotHaveToken)
		}
		if len(context) > 0 {
			return nil, types.NewVmError(types.ErrorResponseForDeploy)
		}
		kind = types.DeployTransactionKind
	} else {
		kind = types.ExecutionTransactionKind
	}

	if dst.ShardId().IsMainShard() {
		return []byte("asyncCall failed: attempt to send transaction to main shard"), ErrTransactionToMainShard
	}

	if uint32(dst.ShardId()) >= nShards {
		return nil, ErrShardIdIsTooBig
	}

	if forwardKind == types.ForwardKindNone {
		if err := withdrawFunds(state, caller.Address(), feeCredit); err != nil {
			return []byte("asyncCall failed: withdrawFunds failed"), err
		}
	}

	if err := withdrawFunds(state, caller.Address(), types.NewValue(value)); err != nil {
		return []byte("asyncCall failed: withdrawFunds failed"), err
	}

	// TODO: We should consider non-refundable transactions
	setRefundTo(&refundTo, state.GetInTransaction())
	setBounceTo(&bounceTo, state.GetInTransaction())

	// Internal is required for the transaction
	payload := types.InternalTransactionPayload{
		Kind:           kind,
		FeeCredit:      feeCredit,
		ForwardKind:    types.ForwardKind(forwardKind),
		Value:          types.NewValue(value),
		Token:          tokens,
		To:             dst,
		RefundTo:       refundTo,
		BounceTo:       bounceTo,
		Data:           input,
		RequestContext: context,
	}
	res = make([]byte, 32)
	res[31] = 1

	_, err = state.AddOutTransaction(caller.Address(), &payload, responseProcessingGas)

	return res, err
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
	if !ok1 || !ok2 || !ok3 || len(sig) != common.SignatureSize {
		return common.EmptyHash[:], nil
	}
	result := crypto.VerifySignature(pubkey, common.BigToHash(hash).Bytes(), sig[:64])
	if result {
		return common.LeftPadBytes([]byte{1}, 32), nil
	}
	return common.EmptyHash[:], nil
}

// arguments: bytes pubkey, uint256 hash, bytes signature
// returns: bool signatureValid
var (
	uint256Ty, _        = abi.NewType("uint256", "", nil)
	bytesTy, _          = abi.NewType("bytes", "", nil)
	verifySignatureArgs = abi.Arguments{
		abi.Argument{Name: "pubkey", Type: bytesTy},
		abi.Argument{Name: "hash", Type: uint256Ty},
		abi.Argument{Name: "signature", Type: bytesTy},
	}
)

func VerifySignatureArgs() abi.Arguments {
	return verifySignatureArgs
}

type checkIsInternal struct{}

var _ ReadOnlyPrecompiledContract = (*checkIsInternal)(nil)

func (c *checkIsInternal) RequiredGas([]byte, StateDBReadOnly) (uint64, error) {
	return 10, nil
}

func (a *checkIsInternal) Run(
	state StateDBReadOnly,
	input []byte,
	value *uint256.Int,
	caller ContractRef,
) ([]byte, error) {
	res := make([]byte, 32)

	if state.IsInternalTransaction() {
		res[31] = 1
	}

	return res, nil
}

func precompiledArgs(method string, input []byte, argCount int) ([]any, error) {
	if len(input) < 4 {
		return nil, types.NewVmError(types.ErrorPrecompileTooShortCallData)
	}

	// Unpack arguments, skipping the first 4 bytes (function selector)
	args, err := getPrecompiledMethod(method).Inputs.Unpack(input[4:])
	if err != nil {
		return nil, err
	}
	if len(args) != argCount {
		return nil, types.NewVmError(types.ErrorPrecompileWrongNumberOfArguments)
	}
	return args, nil
}

type governance struct{}

var _ EvmAccessedPrecompiledContract = (*governance)(nil)

func (g *governance) RequiredGas([]byte, StateDBReadOnly) (uint64, error) {
	return 10, nil
}

func (g *governance) Run(evm *EVM, input []byte, value *uint256.Int, caller ContractRef) ([]byte, error) {
	if caller.Address() != types.GovernanceAddress {
		return nil, types.NewVmError(types.ErrorPrecompileWrongCaller)
	}

	args, err := precompiledArgs("precompileRollback", input, 4)
	if err != nil {
		return nil, err
	}

	version, ok := args[0].(uint32)
	if !ok || version != 1 {
		return nil, types.NewVmError(types.ErrorPrecompileWrongVersion)
	}

	counter, ok1 := args[1].(uint32)
	patchLevel, ok2 := args[2].(uint32)
	mainBlockId, ok3 := args[3].(uint64)
	if !ok1 || !ok2 || !ok3 {
		return nil, types.NewVmError(types.ErrorAbiUnpackFailed)
	}

	if evm.Context.RollbackCounter != counter {
		return nil, types.NewVmError(types.ErrorPrecompileBadArgument)
	}

	err = evm.StateDB.Rollback(counter, patchLevel, mainBlockId)

	res := make([]byte, 32)
	if err == nil {
		res[31] = 1
	}

	return res, err
}

type checkIsResponse struct{}

var _ ReadOnlyPrecompiledContract = (*checkIsResponse)(nil)

func (c *checkIsResponse) RequiredGas([]byte, StateDBReadOnly) (uint64, error) {
	return 10, nil
}

func (a *checkIsResponse) Run(
	state StateDBReadOnly,
	input []byte,
	value *uint256.Int,
	caller ContractRef,
) ([]byte, error) {
	if !state.GetTransactionFlags().IsResponse() {
		return nil, types.NewVmError(types.ErrorOnlyResponseCheckFailed)
	}

	res := make([]byte, 32)
	res[31] = 1

	return res, nil
}

type manageToken struct{}

var _ ReadWritePrecompiledContract = (*manageToken)(nil)

func (c *manageToken) RequiredGas([]byte, StateDBReadOnly) (uint64, error) {
	return 10, nil
}

func (c *manageToken) Run(state StateDB, input []byte, value *uint256.Int, caller ContractRef) ([]byte, error) {
	if len(input) < 4 {
		return nil, types.NewVmError(types.ErrorPrecompileTooShortCallData)
	}

	res := make([]byte, 32)

	args, err := getPrecompiledMethod("precompileManageToken").Inputs.Unpack(input[4:])
	if err != nil {
		return nil, types.NewVmVerboseError(types.ErrorAbiUnpackFailed, err.Error())
	}
	if len(args) != 2 {
		return nil, types.NewVmError(types.ErrorPrecompileWrongNumberOfArguments)
	}

	amountBig, ok := args[0].(*big.Int)
	check.PanicIfNotf(ok, "manageToken failed: `amountBig` is not a big.Int: %v", args[0])
	amount := types.NewValueFromBigMust(amountBig)

	mint, ok := args[1].(bool)
	check.PanicIfNotf(ok, "manageToken failed: `mint` is not a bool: %v", args[1])

	tokenId := types.TokenId(caller.Address())

	action := state.AddToken
	if !mint {
		action = state.SubToken
	}

	if err = action(caller.Address(), tokenId, amount); err != nil {
		actionName := "AddToken"
		if !mint {
			actionName = "SubToken"
		}
		return nil, types.NewVmVerboseError(
			types.ErrorPrecompileWrongNumberOfArguments, fmt.Sprintf("%s failed: %v", actionName, err))
	}

	// Set return data to boolean `true` value
	res[31] = 1

	return res, nil
}

type tokenBalance struct{}

var _ ReadOnlyPrecompiledContract = (*tokenBalance)(nil)

func (c *tokenBalance) RequiredGas([]byte, StateDBReadOnly) (uint64, error) {
	return 10, nil
}

func (a *tokenBalance) Run(
	state StateDBReadOnly,
	input []byte,
	value *uint256.Int,
	caller ContractRef,
) ([]byte, error) {
	if len(input) < 4 {
		return nil, types.NewVmError(types.ErrorPrecompileTooShortCallData)
	}

	res := make([]byte, 32)

	// Unpack arguments, skipping the first 4 bytes (function selector)
	args, err := getPrecompiledMethod("precompileGetTokenBalance").Inputs.Unpack(input[4:])
	if err != nil {
		return nil, types.NewVmVerboseError(types.ErrorAbiUnpackFailed, err.Error())
	}
	if len(args) != 2 {
		return nil, types.NewVmError(types.ErrorPrecompileWrongNumberOfArguments)
	}

	// Get `id` argument
	tokenId, ok := args[0].(types.Address)
	check.PanicIfNotf(ok, "tokenBalance failed: tokenId is not an Address: %v", args[0])

	// Get `addr` argument
	addr, ok := args[1].(types.Address)
	check.PanicIfNotf(ok, "tokenBalance failed: addr argument is not an address")

	if addr == types.EmptyAddress {
		addr = caller.Address()
	} else if addr.ShardId() != caller.Address().ShardId() {
		return nil, types.NewVmVerboseError(types.ErrorCrossShardTransaction, "tokenBalance")
	}

	tokens := state.GetTokens(addr)
	r, ok := tokens[types.TokenId(tokenId)]
	if ok {
		b := r.Bytes32()
		return b[:], nil
	}

	return res, nil
}

type sendTokenSync struct{}

var _ ReadWritePrecompiledContract = (*sendTokenSync)(nil)

func (c *sendTokenSync) RequiredGas([]byte, StateDBReadOnly) (uint64, error) {
	return 10, nil
}

func (c *sendTokenSync) Run(state StateDB, input []byte, value *uint256.Int, caller ContractRef) ([]byte, error) {
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
	check.PanicIfNotf(ok, "sendTokenSync failed: addr argument is not an address")

	if caller.Address().ShardId() != addr.ShardId() {
		return nil, fmt.Errorf("sendTokenSync: %w: %s -> %s",
			ErrCrossShardTransaction, caller.Address().ShardId(), addr.ShardId())
	}

	// Get tokens
	tokens, err := extractTokens(args[1])
	if err != nil {
		return nil, types.NewVmVerboseError(types.ErrorPrecompileInvalidTokenArray, "sendTokenSync")
	}

	state.SetTokenTransfer(tokens)

	res := make([]byte, 32)
	res[31] = 1

	return res, nil
}

type getTransactionTokens struct{}

var _ ReadOnlyPrecompiledContract = (*getTransactionTokens)(nil)

func (c *getTransactionTokens) RequiredGas([]byte, StateDBReadOnly) (uint64, error) {
	return 10, nil
}

func (c *getTransactionTokens) Run(
	state StateDBReadOnly,
	input []byte,
	value *uint256.Int,
	caller ContractRef,
) ([]byte, error) {
	callerTokens := caller.Token()
	res, err := getPrecompiledMethod("precompileGetTransactionTokens").Outputs.Pack(callerTokens)
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

	// Get `transaction` argument
	transaction, ok := args[0].(string)
	if !ok {
		return nil, types.NewVmError(types.ErrorAbiUnpackFailed)
	}

	// Get `data` argument
	slice := reflect.ValueOf(args[1])
	data := make([]types.Uint256, slice.Len())
	for i := range slice.Len() {
		v, ok := slice.Index(i).Interface().(*big.Int)
		if !ok {
			return nil, types.NewVmError(types.ErrorAbiUnpackFailed)
		}
		data[i].SetFromBig(v)
	}

	debugLog, err := types.NewDebugLog([]byte(transaction), data)
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

var consoleLogger = logging.NewLogger("solidity")

type consolePrecompile struct{}

var _ EvmAccessedPrecompiledContract = (*consolePrecompile)(nil)

func (g *consolePrecompile) RequiredGas([]byte, StateDBReadOnly) (uint64, error) {
	return 100, nil
}

func (g *consolePrecompile) Run(evm *EVM, input []byte, value *uint256.Int, caller ContractRef) ([]byte, error) {
	str, err := console.ProcessLog(input)
	if err != nil {
		return nil, types.NewVmVerboseError(types.ErrorConsoleParseInputFailed, err.Error())
	}
	consoleLogger.Info().Int(logging.FieldShardId, int(evm.StateDB.GetShardID())).Msg(str)

	res := make([]byte, 32)
	res[31] = 1

	return res, nil
}
