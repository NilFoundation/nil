package types

import (
	"io"
	"math/big"

	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/holiman/uint256"
)

var (
	Value0   = NewValueFromUint64(0)
	Value10  = NewValueFromUint64(10)
	Value100 = NewValueFromUint64(100)
)

type Value struct{ *Uint256 }

func NewValue(val *uint256.Int) Value {
	v := Uint256(*val)
	return Value{&v}
}

func NewValueFromUint64(val uint64) Value {
	return Value{NewUint256(val)}
}

func NewValueFromDecimal(str string) (Value, error) {
	v, err := NewUint256FromDecimal(str)
	if err != nil {
		return Value{}, err
	}
	return Value{v}, nil
}

func NewZeroValue() Value {
	return Value0
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

func (v Value) Sign() int {
	return v.Uint256.safeInt().Sign()
}

func (v Value) Uint64() uint64 {
	return v.Uint256.safeInt().Uint64()
}

func (v Value) Add(other Value) Value {
	res, overflow := v.AddOverflow(other)
	check.PanicIfNot(!overflow)
	return res
}

func (v Value) Mul(other Value) Value {
	return NewValue(uint256.NewInt(0).Mul(v.Int(), other.Int()))
}

func (v Value) Mul64(other uint64) Value {
	return v.Mul(NewValueFromUint64(other))
}

func (v Value) Div(other Value) Value {
	return NewValue(uint256.NewInt(0).Div(v.Int(), other.Int()))
}

func (v Value) Div64(other uint64) Value {
	return v.Div(NewValueFromUint64(other))
}

func (v Value) AddOverflow(other Value) (Value, bool) {
	res, overflow := v.addOverflow(other.Uint256)
	return Value{res}, overflow
}

func (v Value) Sub(other Value) Value {
	res, overflow := v.SubOverflow(other)
	check.PanicIfNot(!overflow)
	return res
}

func (v Value) Eq(other Value) bool {
	return v.Int().Eq(other.Int())
}

func (v Value) SubOverflow(other Value) (Value, bool) {
	res, overflow := v.subOverflow(other.Uint256)
	return Value{res}, overflow
}

func (v Value) Add64(other uint64) Value {
	res, overflow := v.addOverflow(NewUint256(other))
	check.PanicIfNot(!overflow)
	return Value{res}
}

func (v Value) Sub64(other uint64) Value {
	res, overflow := v.subOverflow(NewUint256(other))
	check.PanicIfNot(!overflow)
	return Value{res}
}

func (v Value) Cmp(other Value) int {
	return v.cmp(other.Uint256)
}

func (v Value) ToGas(price Value) Gas {
	return Gas(v.div64(price.Uint256))
}

func (v Value) ToBig() *big.Int {
	return v.safeInt().ToBig()
}

func (v *Value) MarshalNil() ([]byte, error) {
	return rlp.EncodeToBytes(v)
}

func (v *Value) UnmarshalNil(buf []byte) error {
	return rlp.DecodeBytes(buf, v)
}

// EncodeRLP rlp marshals the Value object to a target array
func (v Value) EncodeRLP(w io.Writer) error {
	if v.Uint256 == nil {
		return Value0.EncodeRLP(w)
	}
	return v.Uint256.EncodeRLP(w)
}

// DecodeRLP rlp unmarshals the Value object
func (v *Value) DecodeRLP(r *rlp.Stream) error {
	var u Uint256
	if err := u.DecodeRLP(r); err != nil {
		return err
	}
	v.Uint256 = &u
	return nil
}

func (v *Value) UnmarshalJSON(input []byte) error {
	v.Uint256 = new(Uint256)
	return v.Uint256.UnmarshalJSON(input)
}

func (v *Value) UnmarshalText(input []byte) error {
	v.Uint256 = new(Uint256)
	return v.Uint256.UnmarshalText(input)
}

func (v Value) MarshalJSON() ([]byte, error) {
	hex := v.safeInt().Hex()
	return []byte(`"` + hex + `"`), nil
}

func (v Value) MarshalText() ([]byte, error) {
	return v.safeInt().MarshalText()
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
