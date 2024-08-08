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
	"github.com/NilFoundation/nil/nil/internal/contracts"
	"github.com/NilFoundation/nil/nil/internal/tracing"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	eth_common "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"
	"github.com/rs/zerolog/log"
)

func init() {
	check.PanicIfNot(eth_common.AddressLength == types.AddrSize)
}

// PrecompiledContract is the basic interface for native Go contracts. The implementation
// requires a deterministic gas count based on the input size of the Run method of the
// contract.
type PrecompiledContract interface {
	// RequiredPrice calculates the contract gas use
	RequiredGas(input []byte) uint64
}

type ReadOnlyPrecompiledContract interface {
	// Run runs the precompiled contract
	Run(state StateDBReadOnly, input []byte, value *uint256.Int, caller ContractRef) ([]byte, error)
}

type ReadWritePrecompiledContract interface {
	// Run runs the precompiled contract without state modifications
	Run(state StateDB, input []byte, value *uint256.Int, caller ContractRef) ([]byte, error)
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
	MintCurrencyAddress    = types.BytesToAddress([]byte{0xd0})
	CurrencyBalanceAddress = types.BytesToAddress([]byte{0xd1})
	SendTokensAddress      = types.BytesToAddress([]byte{0xd2})
	MessageTokensAddress   = types.BytesToAddress([]byte{0xd3})
	GetGasPriceAddress     = types.BytesToAddress([]byte{0xd4})
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
	MintCurrencyAddress:    &mintCurrency{},
	CurrencyBalanceAddress: &currencyBalance{},
	SendTokensAddress:      &sendCurrencySync{},
	MessageTokensAddress:   &getMessageTokens{},
	GetGasPriceAddress:     &getGasPrice{},
}

// RunPrecompiledContract runs and evaluates the output of a precompiled contract.
// It returns
// - the returned bytes,
// - the _remaining_ gas,
// - any error that occurred
func RunPrecompiledContract(p PrecompiledContract, state StateDB, input []byte, suppliedGas uint64,
	logger *tracing.Hooks, value *uint256.Int, caller ContractRef, readOnly bool,
) (ret []byte, remainingGas uint64, err error) {
	gasCost := p.RequiredGas(input)
	if suppliedGas < gasCost {
		return nil, 0, fmt.Errorf("%w: %d < %d", ErrOutOfGas, suppliedGas, gasCost)
	}
	if logger != nil && logger.OnGasChange != nil {
		logger.OnGasChange(suppliedGas, suppliedGas-gasCost, tracing.GasChangeCallPrecompiledContract)
	}
	suppliedGas -= gasCost
	switch p := p.(type) {
	case ReadOnlyPrecompiledContract:
		ret, err = p.Run(StateDBReadOnly(state), input, value, caller)
	case ReadWritePrecompiledContract:
		if readOnly {
			err = ErrWriteProtection
		} else {
			ret, err = p.Run(state, input, value, caller)
		}
	default:
		err = ErrUnexpectedType
	}
	return ret, suppliedGas, err
}

type simple struct {
	contract SimplePrecompiledContract
}

func (a *simple) RequiredGas(input []byte) uint64 {
	return a.contract.RequiredGas(input)
}

func (a *simple) Run(_ StateDBReadOnly /* state */, input []byte, _ *uint256.Int /* value */, _ ContractRef /* caller */) ([]byte, error) {
	return a.contract.Run(input)
}

type sendRawMessage struct{}

// TODO: Make this dynamically calculated based on the network conditions and current shard gas price
const ForwardFee uint64 = 1_000

func (c *sendRawMessage) RequiredGas([]byte) uint64 {
	return ForwardFee
}

func setRefundTo(refundTo *types.Address, msg *types.Message) {
	if msg == nil {
		return
	}
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
	check.PanicIfNot(ok)
	return method
}

func (c *sendRawMessage) Run(state StateDB, input []byte, value *uint256.Int, caller ContractRef) ([]byte, error) {
	payload := new(types.InternalMessagePayload)
	if err := payload.UnmarshalSSZ(input); err != nil {
		return nil, err
	}

	if payload.To.ShardId().IsMainShard() {
		return nil, ErrMessageToMainShard
	}

	if err := withdrawFunds(state, caller.Address(), payload.Value); err != nil {
		return nil, err
	}

	if err := withdrawFunds(state, caller.Address(), payload.FeeCredit); err != nil {
		return nil, err
	}

	// TODO: We should consider non-refundable messages
	setRefundTo(&payload.RefundTo, state.GetInMessage())
	setBounceTo(&payload.BounceTo, state.GetInMessage())

	log.Logger.Debug().Msgf("sendRawMessage to: %s\n", payload.To.Hex())

	_, err := state.AddOutInternal(caller.Address(), payload)

	return nil, err
}

type asyncCall struct{}

func (c *asyncCall) RequiredGas([]byte) uint64 {
	return ForwardFee
}

func convertGethAddress(addrGeth any) (types.Address, error) {
	dstGo, ok := addrGeth.(eth_common.Address)
	if !ok {
		return types.EmptyAddress, fmt.Errorf("dst is not a common.Address: %v", addrGeth)
	}
	return types.BytesToAddress(dstGo.Bytes()), nil
}

func extractCurrencies(arg any) ([]types.CurrencyBalance, error) {
	slice := reflect.ValueOf(arg)
	currencies := make([]types.CurrencyBalance, slice.Len())
	for i := range slice.Len() {
		elem := slice.Index(i)
		id, ok := elem.FieldByIndex([]int{0}).Interface().(*big.Int)
		if !ok {
			return nil, errors.New("currencyId is not a big.Int")
		}
		currencyId, _ := uint256.FromBig(id)
		currencies[i].Currency = currencyId.Bytes32()

		balanceBig, ok := elem.FieldByIndex([]int{1}).Interface().(*big.Int)
		if !ok {
			return nil, errors.New("balance is not a big.Int")
		}
		currencies[i].Balance = types.NewValueFromBigMust(balanceBig)
	}
	return currencies, nil
}

func (c *asyncCall) Run(state StateDB, input []byte, value *uint256.Int, caller ContractRef) (res []byte, err error) {
	res = make([]byte, 32)

	// Unpack arguments, skipping the first 4 bytes (function selector)
	args, err := getPrecompiledMethod("precompileAsyncCall").Inputs.Unpack(input[4:])
	if err != nil {
		log.Logger.Error().Err(err).Msg("precompileAsyncCall pack failed")
		return res, nil
	}
	if len(args) != 8 {
		return res, errors.New("precompileAsyncCall: invalid number of arguments")
	}

	// Get `isDeploy` argument
	deploy, ok := args[0].(bool)
	if !ok {
		log.Logger.Error().Msg("deploy is not a bool")
		return res, nil
	}

	// Get `forwardKind` argument
	forwardKind, ok := args[1].(uint8)
	if !ok {
		log.Logger.Error().Msgf("forwardKind is not an uint8: %T", args[1])
		return res, nil
	}

	// Get `dst` argument
	dst, err := convertGethAddress(args[2])
	if err != nil {
		log.Logger.Error().Err(err).Msgf("dst is not an address: %T", args[2])
		return res, nil
	}

	// Get `refundTo` argument
	refundTo, err := convertGethAddress(args[3])
	if err != nil {
		log.Logger.Error().Err(err).Msgf("refundTo is not an address: %T", args[3])
		return res, nil
	}

	// Get `bounceTo` argument
	bounceTo, err := convertGethAddress(args[4])
	if err != nil {
		log.Logger.Error().Err(err).Msgf("bounceTo is not an address: %T", args[4])
		return res, nil
	}

	// Get `messageGas` argument
	messageGasBig, ok := args[5].(*big.Int)
	if !ok {
		log.Logger.Error().Msgf("messageGas is not a big.Int: %T", args[5])
		return res, nil
	}
	messageGas, overflow := types.NewValueFromBig(messageGasBig)
	if overflow {
		log.Logger.Error().Msgf("messageGas overflow: %v", messageGasBig)
		return res, nil
	}

	// Get `currencies` argument, which is a slice of `CurrencyBalance`
	currencies, err := extractCurrencies(args[6])
	if err != nil {
		log.Logger.Error().Err(err).Msgf("currencies is not a slice of CurrencyBalance: %T", args[6])
		return res, nil
	}

	// Get `input` argument
	if input, ok = args[7].([]byte); !ok {
		log.Logger.Error().Msg("input is not a byte slice")
		return res, nil
	}

	var kind types.MessageKind
	if deploy {
		kind = types.DeployMessageKind
	} else {
		kind = types.ExecutionMessageKind
	}

	if dst.ShardId().IsMainShard() {
		return nil, ErrMessageToMainShard
	}

	if forwardKind == types.ForwardKindNone {
		if err := withdrawFunds(state, caller.Address(), messageGas); err != nil {
			return res, err
		}
	}

	if err := withdrawFunds(state, caller.Address(), types.NewValue(value)); err != nil {
		return res, err
	}

	// TODO: We should consider non-refundable messages
	setRefundTo(&refundTo, state.GetInMessage())
	setBounceTo(&bounceTo, state.GetInMessage())

	// Internal is required for the message
	payload := types.InternalMessagePayload{
		Kind:        kind,
		FeeCredit:   messageGas,
		ForwardKind: types.ForwardKind(forwardKind),
		Value:       types.NewValue(value),
		Currency:    currencies,
		To:          dst,
		RefundTo:    refundTo,
		BounceTo:    bounceTo,
		Data:        slices.Clone(input),
	}
	res = make([]byte, 32)
	res[31] = 1

	_, err = state.AddOutInternal(caller.Address(), &payload)

	return res, err
}

type verifySignature struct{}

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

func (c *checkIsInternal) RequiredGas([]byte) uint64 {
	return 10
}

func (a *checkIsInternal) Run(state StateDBReadOnly, input []byte, value *uint256.Int, caller ContractRef) ([]byte, error) {
	res := make([]byte, 32)

	if state.IsInternalMessage() {
		res[31] = 1
	}

	return res, nil
}

type mintCurrency struct{}

func (c *mintCurrency) RequiredGas([]byte) uint64 {
	return 10
}

func (c *mintCurrency) Run(state StateDB, input []byte, value *uint256.Int, caller ContractRef) ([]byte, error) {
	res := make([]byte, 32)

	if caller.Address() != types.MinterAddress {
		return res, nil
	}

	args, err := getPrecompiledMethod("precompileMintCurrency").Inputs.Unpack(input[4:])
	if err != nil {
		return nil, err
	}
	if len(args) != 2 {
		return nil, errors.New("precompileMintCurrency: invalid number of arguments")
	}

	currencyIdBig, ok1 := args[0].(*big.Int)
	amountBig, ok2 := args[1].(*big.Int)
	if !ok1 || !ok2 {
		return nil, errors.New("currencyId or amount is not a big.Int")
	}
	amount, overflow := types.NewValueFromBig(amountBig)
	if overflow {
		log.Logger.Warn().Msgf("amount overflow: %v", amountBig)
	}

	var currencyId types.CurrencyId
	currencyIdBig.FillBytes(currencyId[:])

	if err = state.AddCurrency(caller.Address(), currencyId, amount); err != nil {
		return nil, err
	}

	// Set return data to boolean `true` value
	res[31] = 1

	return res, nil
}

type currencyBalance struct{}

func (c *currencyBalance) RequiredGas([]byte) uint64 {
	return 10
}

func (a *currencyBalance) Run(state StateDBReadOnly, input []byte, value *uint256.Int, caller ContractRef) ([]byte, error) {
	res := make([]byte, 32)

	// Unpack arguments, skipping the first 4 bytes (function selector)
	args, err := getPrecompiledMethod("precompileGetCurrencyBalance").Inputs.Unpack(input[4:])
	if err != nil {
		return nil, err
	}
	if len(args) != 2 {
		return nil, errors.New("precompileGetCurrencyBalance: invalid number of arguments")
	}

	// Get `id` argument
	currencyIdBig, ok := args[0].(*big.Int)
	if !ok {
		return nil, errors.New("currencyId is not a big.Int")
	}

	// Get `addr` argument
	addr, err := convertGethAddress(args[1])
	if err != nil {
		return nil, err
	}
	if addr == types.EmptyAddress {
		addr = caller.Address()
	} else if addr.ShardId() != caller.Address().ShardId() {
		return nil, fmt.Errorf("%w: currencyBalance: cross shard is not allowed", ErrExecutionReverted)
	}

	var currencyId types.CurrencyId
	currencyIdBig.FillBytes(currencyId[:])

	currencies := state.GetCurrencies(addr)
	r, ok := currencies[currencyId]
	if ok {
		b := r.Bytes32()
		return b[:], nil
	}

	return res, nil
}

type sendCurrencySync struct{}

var _ PrecompiledContract = (*sendCurrencySync)(nil)

func (c *sendCurrencySync) RequiredGas([]byte) uint64 {
	return 10
}

func (c *sendCurrencySync) Run(state StateDB, input []byte, value *uint256.Int, caller ContractRef) ([]byte, error) {
	// Unpack arguments, skipping the first 4 bytes (function selector)
	args, err := getPrecompiledMethod("precompileSendTokens").Inputs.Unpack(input[4:])
	if err != nil {
		return nil, err
	}
	if len(args) != 2 {
		return nil, errors.New("precompileSendTokens: invalid number of arguments")
	}

	// Get destination address
	addr, err := convertGethAddress(args[0])
	if err != nil {
		return nil, err
	}
	if caller.Address().ShardId() != addr.ShardId() {
		return nil, fmt.Errorf("sendCurrencySync: %w: %s -> %s",
			ErrCrossShardMessage, caller.Address().ShardId(), addr.ShardId())
	}

	// Get currencies
	currencies, err := extractCurrencies(args[1])
	if err != nil {
		return nil, err
	}

	state.SetCurrencyTransfer(currencies)

	res := make([]byte, 32)
	res[31] = 1

	return res, nil
}

type getMessageTokens struct{}

var _ PrecompiledContract = (*getMessageTokens)(nil)

func (c *getMessageTokens) RequiredGas([]byte) uint64 {
	return 10
}

func (c *getMessageTokens) Run(state StateDBReadOnly, input []byte, value *uint256.Int, caller ContractRef) ([]byte, error) {
	callerCurrencies := caller.Currency()
	abiCurrencies := make([]types.CurrencyBalanceAbiCompatible, len(callerCurrencies))
	for i, c := range callerCurrencies {
		abiCurrencies[i].Currency = new(big.Int).SetBytes(c.Currency[:])
		abiCurrencies[i].Balance = c.Balance.ToBig()
	}

	res, err := getPrecompiledMethod("precompileGetMessageTokens").Outputs.Pack(abiCurrencies)
	if err != nil {
		return nil, err
	}

	return res, nil
}

type getGasPrice struct{}

var _ PrecompiledContract = (*getGasPrice)(nil)

func (c *getGasPrice) RequiredGas([]byte) uint64 {
	return 10
}

func (c *getGasPrice) Run(state StateDBReadOnly, input []byte, value *uint256.Int, caller ContractRef) ([]byte, error) {
	method := getPrecompiledMethod("precompileGetGasPrice")

	args, err := method.Inputs.Unpack(input[4:])
	if err != nil {
		return nil, err
	}
	if len(args) != 1 {
		return nil, errors.New("precompileGetGasPrice: invalid number of arguments")
	}

	// Get `shardId` argument
	shardId, ok := args[0].(*big.Int)
	if !ok {
		return nil, errors.New("shardId is not a big.Int")
	}
	if !shardId.IsUint64() {
		return nil, errors.New("shardId is too big")
	}

	gasPrice, err := state.GetGasPrice(types.ShardId(shardId.Uint64()))
	if err != nil {
		return nil, err
	}

	res, err := method.Outputs.Pack(gasPrice.ToBig())
	if err != nil {
		return nil, err
	}

	return res, nil
}
