package types

import (
	"math/big"

	ssz "github.com/NilFoundation/fastssz"
	"github.com/NilFoundation/nil/common/check"
	"github.com/holiman/uint256"
)

type Value struct{ *Uint256 }

func NewValue(val *uint256.Int) Value {
	v := Uint256(*val)
	return Value{&v}
}

func NewValueFromUint64(val uint64) Value {
	return Value{NewUint256(val)}
}

func NewValueFromBig(val *big.Int) (Value, bool) {
	res, overflow := uint256.FromBig(val)
	if overflow {
		return Value{}, true
	}
	return Value{(*Uint256)(res)}, false
}

func NewValueFromBigMust(val *big.Int) Value {
	res, overflow := NewValueFromBig(val)
	check.PanicIfNot(!overflow)
	return res
}

func NewValueFromBytes(input []byte) Value {
	return Value{NewUint256FromBytes(input)}
}

func (v Value) IsZero() bool {
	return v.Uint256 == nil || v.Uint256.IsZero()
}

func (v Value) Add(other Value) Value {
	return Value{v.Uint256.add(other.Uint256)}
}

func (v Value) Sub(other Value) Value {
	return Value{v.Uint256.sub(other.Uint256)}
}

func (v Value) Add64(other uint64) Value {
	return Value{v.Uint256.add(NewUint256(other))}
}

func (v Value) Sub64(other uint64) Value {
	return Value{v.Uint256.sub(NewUint256(other))}
}

func (v Value) Cmp(other Value) int {
	return v.Uint256.cmp(other.Uint256)
}

func (v Value) ToGas(price Value) Gas {
	return Gas(v.Uint256.div64(price.Uint256))
}

func (v Value) ToBig() *big.Int {
	return v.safeInt().ToBig()
}

// We need to override SSZ methods, because fast-ssz does not support wrappers around pointer types.

func (v *Value) MarshalSSZ() ([]byte, error) {
	return v.safeInt().MarshalSSZ()
}

func (v *Value) MarshalSSZTo(dst []byte) ([]byte, error) {
	return v.safeInt().MarshalSSZAppend(dst)
}

func (v *Value) UnmarshalSSZ(buf []byte) error {
	v.Uint256 = new(Uint256)
	return v.Uint256.UnmarshalSSZ(buf)
}

func (v *Value) SizeSSZ() (size int) {
	return v.safeInt().SizeSSZ()
}

func (v *Value) HashTreeRoot() ([32]byte, error) {
	b, _ := v.MarshalSSZTo(make([]byte, 0, 32)) // ignore error, cannot fail
	var hash [32]byte
	copy(hash[:], b)
	return hash, nil
}

func (v *Value) HashTreeRootWith(hh ssz.HashWalker) (err error) {
	bytes, _ := v.MarshalSSZTo(make([]byte, 0, 32)) // ignore error, cannot fail
	hh.AppendBytes32(bytes)
	return
}

func (v *Value) GetTree() (*ssz.Node, error) {
	return ssz.ProofTree(v)
}

func (v *Value) UnmarshalJSON(input []byte) error {
	v.Uint256 = new(Uint256)
	return v.Uint256.UnmarshalJSON(input)
}

func (v *Value) UnmarshalText(input []byte) error {
	v.Uint256 = new(Uint256)
	return v.Uint256.UnmarshalText(input)
}

func (v *Value) Set(value string) error {
	v.Uint256 = new(Uint256)
	return v.Uint256.Set(value)
}

func (v Value) String() string {
	return v.safeInt().String()
}

func (Value) Type() string {
	return "Value"
}
