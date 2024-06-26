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

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/check"
	"github.com/NilFoundation/nil/contracts"
	"github.com/NilFoundation/nil/core/tracing"
	"github.com/NilFoundation/nil/core/types"
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

	// Run runs the precompiled contract
	Run(state StateDB, input []byte, gas uint64, value *uint256.Int, caller ContractRef, _ bool) ([]byte, error)
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
)

// PrecompiledContractsPrague contains the set of pre-compiled Ethereum
// contracts used in the Prague release.
var PrecompiledContractsPrague = map[types.Address]PrecompiledContract{
	types.BytesToAddress([]byte{0x01}): &simple{&ecrecover{}},
	types.BytesToAddress([]byte{0x02}): &sha256hash{},
	types.BytesToAddress([]byte{0x03}): &simple{&ripemd160hash{}},
	types.BytesToAddress([]byte{0x04}): &dataCopy{},
	types.BytesToAddress([]byte{0x05}): &bigModExp{eip2565: true},
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
	VerifySignatureAddress: &verifySignature{},
	CheckIsInternalAddress: &checkIsInternal{},
	MintCurrencyAddress:    &mintCurrency{},
	CurrencyBalanceAddress: &currencyBalance{},
}

// RunPrecompiledContract runs and evaluates the output of a precompiled contract.
// It returns
// - the returned bytes,
// - the _remaining_ gas,
// - any error that occurred
func RunPrecompiledContract(p PrecompiledContract, state StateDB, input []byte, suppliedGas uint64,
	logger *tracing.Hooks, gas uint64, value *uint256.Int, caller ContractRef, readOnly bool,
) (ret []byte, remainingGas uint64, err error) {
	gasCost := p.RequiredGas(input)
	if suppliedGas < gasCost {
		return nil, 0, ErrOutOfGas
	}
	if logger != nil && logger.OnGasChange != nil {
		logger.OnGasChange(suppliedGas, suppliedGas-gasCost, tracing.GasChangeCallPrecompiledContract)
	}
	suppliedGas -= gasCost
	output, err := p.Run(state, input, gas, value, caller, readOnly)
	return output, suppliedGas, err
}

type simple struct {
	contract SimplePrecompiledContract
}

func (a *simple) RequiredGas(input []byte) uint64 {
	return a.contract.RequiredGas(input)
}

func (a *simple) Run(state StateDB, input []byte, gas uint64, value *uint256.Int, caller ContractRef, readOnly bool) ([]byte, error) {
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

func withdrawFunds(state StateDB, addr types.Address, value *uint256.Int) error {
	balance, err := state.GetBalance(addr)
	if err != nil {
		return err
	}
	if balance.Lt(value) {
		log.Logger.Error().Msgf("withdrawFunds failed: insufficient balance on address %v, expected at least %v, got %v", addr, value, balance)
		return ErrInsufficientBalance
	}
	return state.SubBalance(addr, value, tracing.BalanceDecreasePrecompile)
}

func (c *sendRawMessage) Run(state StateDB, input []byte, gas uint64, value *uint256.Int, caller ContractRef, readOnly bool) ([]byte, error) {
	if readOnly {
		return nil, ErrWriteProtection
	}
	payload := new(types.InternalMessagePayload)
	if err := payload.UnmarshalSSZ(input); err != nil {
		return nil, err
	}
	if err := withdrawFunds(state, caller.Address(), &payload.Value.Int); err != nil {
		return nil, err
	}

	// TODO: We should consider non-refundable messages
	setRefundTo(&payload.RefundTo, state.GetInMessage())
	setBounceTo(&payload.BounceTo, state.GetInMessage())

	log.Logger.Debug().Msgf("sendRawMessage to: %s\n", payload.To.Hex())

	return nil, AddOutInternal(state, caller.Address(), payload)
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

func (c *asyncCall) Run(state StateDB, input []byte, gas uint64, value *uint256.Int, caller ContractRef, readOnly bool) (res []byte, err error) {
	if readOnly {
		return nil, ErrWriteProtection
	}

	abi, err := contracts.GetAbi("Precompile")
	if err != nil {
		return nil, err
	}
	method, ok := abi.Methods["precompileAsyncCall"]
	if !ok {
		return nil, errors.New("'precompileAsyncCall' method not found in 'Precompile' class")
	}

	// Unpack arguments, skipping the first 4 bytes (function selector)
	args, err := method.Inputs.Unpack(input[4:])
	if err != nil {
		return nil, err
	}

	// Get `isDeploy` argument
	deploy, ok := args[0].(bool)
	if !ok {
		return nil, errors.New("deploy is not a bool")
	}

	// Get `dst` argument
	dst, err := convertGethAddress(args[1])
	if err != nil {
		return nil, err
	}

	// Get `refundTo` argument
	refundTo, err := convertGethAddress(args[2])
	if err != nil {
		return nil, err
	}

	// Get `bounceTo` argument
	bounceTo, err := convertGethAddress(args[3])
	if err != nil {
		return nil, err
	}

	// Get `messageGas` argument
	messageGasBig, ok := args[4].(*big.Int)
	if !ok {
		return nil, errors.New("messageGas is not a big.Int")
	}
	messageGas, _ := uint256.FromBig(messageGasBig)

	// Get `currencies` argument, which is a slice of `CurrencyBalance`
	slice := reflect.ValueOf(args[5])
	currencies := make([]types.CurrencyBalance, slice.Len())
	for i := range slice.Len() {
		elem := slice.Index(i)
		id, ok := elem.FieldByIndex([]int{0}).Interface().(*big.Int)
		if !ok {
			return nil, errors.New("currencyId is not a big.Int")
		}
		currencyId, _ := uint256.FromBig(id)
		currencies[i].Currency = currencyId.Bytes32()

		balance, ok := elem.FieldByIndex([]int{1}).Interface().(*big.Int)
		if !ok {
			return nil, errors.New("balance is not a big.Int")
		}
		b, _ := uint256.FromBig(balance)
		currencies[i].Balance = types.Uint256{Int: *b}
	}

	// Get `input` argument
	if input, ok = args[6].([]byte); !ok {
		return nil, errors.New("input is not a byte slice")
	}

	var kind types.MessageKind
	if deploy {
		kind = types.DeployMessageKind
	} else {
		kind = types.ExecutionMessageKind
	}

	if err := withdrawFunds(state, caller.Address(), messageGas); err != nil {
		return nil, err
	}

	// TODO: We should consider non-refundable messages
	setRefundTo(&refundTo, state.GetInMessage())
	setBounceTo(&bounceTo, state.GetInMessage())

	// Internal is required for the message
	payload := types.InternalMessagePayload{
		Kind:     kind,
		GasLimit: types.Uint256{Int: *messageGas},
		Value:    types.Uint256{Int: *value},
		Currency: currencies,
		To:       dst,
		RefundTo: refundTo,
		BounceTo: bounceTo,
		Data:     slices.Clone(input),
	}
	res = make([]byte, 32)
	res[31] = 1

	return res, AddOutInternal(state, caller.Address(), &payload)
}

func AddOutInternal(state StateDB, caller types.Address, payload *types.InternalMessagePayload) error {
	seqno, err := state.GetSeqno(caller)
	if err != nil {
		return err
	}
	if seqno+1 < seqno {
		return ErrNonceUintOverflow
	}
	if err := state.SetSeqno(caller, seqno+1); err != nil {
		return err
	}

	msg := payload.ToMessage(caller, seqno)

	return state.AddOutMessage(msg)
}

type verifySignature struct{}

func (c *verifySignature) RequiredGas([]byte) uint64 {
	return 5000
}

func (c *verifySignature) Run(state StateDB, input []byte, gas uint64, value *uint256.Int, caller ContractRef, readOnly bool) ([]byte, error) {
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

func (c *checkIsInternal) Run(state StateDB, input []byte, gas uint64, value *uint256.Int, caller ContractRef, readOnly bool) ([]byte, error) {
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

func (c *mintCurrency) Run(state StateDB, input []byte, gas uint64, value *uint256.Int, caller ContractRef, readOnly bool) ([]byte, error) {
	res := make([]byte, 32)

	if caller.Address() != types.MinterAddress {
		return res, nil
	}

	abi, err := contracts.GetAbi("Precompile")
	if err != nil {
		return nil, err
	}
	method, ok := abi.Methods["precompileMintCurrency"]
	if !ok {
		return nil, errors.New("'precompileMintCurrency' method not found in 'Precompile' class")
	}
	args, err := method.Inputs.Unpack(input[4:])
	if err != nil {
		return nil, err
	}
	currencyIdBig, ok1 := args[0].(*big.Int)
	amountBig, ok2 := args[1].(*big.Int)
	if !ok1 || !ok2 {
		return nil, errors.New("currencyId or amount is not a big.Int")
	}
	amount, overflow := uint256.FromBig(amountBig)
	if overflow {
		log.Logger.Warn().Msgf("amount overflow: %v", amountBig)
	}

	var currencyId types.CurrencyId
	currencyIdBig.FillBytes(currencyId[:])

	if err = state.AddCurrency(caller.Address(), &currencyId, amount); err != nil {
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

func (c *currencyBalance) Run(state StateDB, input []byte, gas uint64, value *uint256.Int, caller ContractRef, readOnly bool) ([]byte, error) {
	res := make([]byte, 32)

	if caller.Address() != types.MinterAddress {
		return res, nil
	}

	abi, err := contracts.GetAbi("Precompile")
	if err != nil {
		return nil, err
	}
	method, ok := abi.Methods["precompileGetCurrencyBalance"]
	if !ok {
		return nil, errors.New("'precompileGetCurrencyBalance' method not found in 'Precompile' class")
	}

	// Unpack arguments, skipping the first 4 bytes (function selector)
	args, err := method.Inputs.Unpack(input[4:])
	if err != nil {
		return nil, err
	}

	currencyIdBig, ok := args[0].(*big.Int)
	if !ok {
		return nil, errors.New("currencyId is not a big.Int")
	}

	var currencyId types.CurrencyId
	currencyIdBig.FillBytes(currencyId[:])

	currencies := state.GetCurrencies(caller.Address())
	for _, cur := range currencies {
		if cur.Currency == currencyId {
			r := cur.Balance.Bytes32()
			return r[:], nil
		}
	}

	return res, nil
}
