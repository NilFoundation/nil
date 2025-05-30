package types

import (
	"crypto/rand"
	"database/sql/driver"
	"encoding"
	"encoding/json"
	"io"
	"math/big"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/internal/serialization"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/holiman/uint256"
)

// interfaces
var (
	_ serialization.NilMarshaler   = (*Uint256)(nil)
	_ serialization.NilUnmarshaler = (*Uint256)(nil)
	_ json.Marshaler               = (*Uint256)(nil)
	_ encoding.BinaryMarshaler     = (*Uint256)(nil)
	_ driver.Valuer                = (*Uint256)(nil)
	_ encoding.TextMarshaler       = (*Uint256)(nil)
	_ encoding.TextUnmarshaler     = (*Uint256)(nil)
	_ rlp.Encoder                  = (*Uint256)(nil)
	_ rlp.Decoder                  = (*Uint256)(nil)
)

type Uint256 uint256.Int

func NewUint256(val uint64) *Uint256 {
	return (*Uint256)(uint256.NewInt(val))
}

func NewUint256FromBytes(buf []byte) *Uint256 {
	return (*Uint256)(new(uint256.Int).SetBytes(buf))
}

func NewUint256FromDecimal(str string) (*Uint256, error) {
	v := new(uint256.Int)
	if err := v.SetFromDecimal(str); err != nil {
		return nil, err
	}
	return (*Uint256)(v), nil
}

func NewUint256Random() *Uint256 {
	buf := make([]byte, 32)
	_, err := rand.Read(buf)
	check.PanicIfErr(err)
	return NewUint256FromBytes(buf)
}

func CastToUint256(val *uint256.Int) *Uint256 {
	return (*Uint256)(val)
}

func (u *Uint256) Int() *uint256.Int {
	if u == nil {
		return &uint256.Int{}
	}
	return common.CopyPtr((*uint256.Int)(u))
}

func (u *Uint256) safeInt() *uint256.Int {
	if u == nil {
		return &uint256.Int{}
	}
	return (*uint256.Int)(u)
}

func (u *Uint256) ToBig() *big.Int {
	return (*uint256.Int)(u).ToBig()
}

func (u *Uint256) SetFromBig(b *big.Int) bool {
	return (*uint256.Int)(u).SetFromBig(b)
}

// EncodeRLP rlp marshals the Uint256 object to a target array
func (u Uint256) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, u.safeInt())
}

// DecodeRLP rlp unmarshals the Uint256 object
func (u *Uint256) DecodeRLP(r *rlp.Stream) error {
	var tmp uint256.Int
	if err := r.Decode(&tmp); err != nil {
		return err
	}
	*u = Uint256(tmp)
	return nil
}

func (u *Uint256) MarshalNil() ([]byte, error) {
	return rlp.EncodeToBytes(u)
}

func (u *Uint256) UnmarshalNil(buf []byte) error {
	return rlp.DecodeBytes(buf, u)
}

func (u Uint256) MarshalJSON() ([]byte, error) {
	return u.safeInt().MarshalJSON()
}

func (u *Uint256) UnmarshalJSON(input []byte) error {
	return (*uint256.Int)(u).UnmarshalJSON(input)
}

func (u Uint256) MarshalText() ([]byte, error) {
	return u.safeInt().MarshalText()
}

func (u *Uint256) UnmarshalText(input []byte) error {
	return (*uint256.Int)(u).UnmarshalText(input)
}

func (u *Uint256) MarshalBinary() (data []byte, err error) {
	return u.MarshalNil()
}

func (u Uint256) Value() (driver.Value, error) {
	return u.safeInt().ToBig(), nil
}

func (u Uint256) String() string {
	return u.safeInt().String()
}

func (u *Uint256) Set(value string) error {
	return (*uint256.Int)(u).SetFromDecimal(value)
}

func (u *Uint256) Uint64() uint64 {
	return u.safeInt().Uint64()
}

func (u *Uint256) IsUint64() bool {
	return u.safeInt().IsUint64()
}

func (u *Uint256) Bytes() []byte {
	return u.safeInt().Bytes()
}

func (u *Uint256) Bytes32() [32]byte {
	return u.safeInt().Bytes32()
}

func (u *Uint256) IsZero() bool {
	return u.safeInt().IsZero()
}

func (*Uint256) Type() string {
	return "Uint256"
}

func (u *Uint256) addOverflow(other *Uint256) (*Uint256, bool) {
	res, overflow := uint256.NewInt(0).AddOverflow(u.safeInt(), other.safeInt())
	return (*Uint256)(res), overflow
}

func (u *Uint256) subOverflow(other *Uint256) (*Uint256, bool) {
	res, overflow := uint256.NewInt(0).SubOverflow(u.safeInt(), other.safeInt())
	return (*Uint256)(res), overflow
}

func (u *Uint256) mulOverflow64(other uint64) (*Uint256, bool) {
	res, overflow := uint256.NewInt(0).MulOverflow(u.safeInt(), uint256.NewInt(other))
	return (*Uint256)(res), overflow
}

func (u *Uint256) div64(other *Uint256) uint64 {
	return uint256.NewInt(0).Div(u.safeInt(), other.safeInt()).Uint64()
}

func (u *Uint256) cmp(other *Uint256) int {
	return u.safeInt().Cmp(other.safeInt())
}
