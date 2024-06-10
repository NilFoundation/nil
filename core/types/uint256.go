package types

import (
	"database/sql/driver"
	"encoding"
	"encoding/binary"

	ssz "github.com/NilFoundation/fastssz"
	"github.com/holiman/uint256"
)

// interfaces
var (
	_ ssz.Marshaler            = (*Uint256)(nil)
	_ ssz.Unmarshaler          = (*Uint256)(nil)
	_ ssz.HashRoot             = (*Uint256)(nil)
	_ encoding.BinaryMarshaler = (*Uint256)(nil)
	_ driver.Valuer            = (*Uint256)(nil)
)

type Uint256 struct{ uint256.Int }

func NewUint256(val uint64) *Uint256 {
	return &Uint256{*uint256.NewInt(val)}
}

// TODO: it can be simplified after following issues will be resolved
//   - https://github.com/holiman/uint256/pull/171
//   - https://github.com/holiman/uint256/issues/170
//
// MarshalSSZ ssz marshals the Uint256 object
func (u *Uint256) MarshalSSZ() ([]byte, error) {
	blob, _ := u.MarshalSSZTo(make([]byte, 0, 32))
	return blob, nil
}

// MarshalSSZTo ssz marshals the Uint256 object to a target array
func (u *Uint256) MarshalSSZTo(dst []byte) ([]byte, error) {
	dst = binary.LittleEndian.AppendUint64(dst, u.Int[0])
	dst = binary.LittleEndian.AppendUint64(dst, u.Int[1])
	dst = binary.LittleEndian.AppendUint64(dst, u.Int[2])
	dst = binary.LittleEndian.AppendUint64(dst, u.Int[3])
	return dst, nil
}

// UnmarshalSSZ ssz unmarshals the Uint256 object
func (u *Uint256) UnmarshalSSZ(buf []byte) error {
	return u.Int.UnmarshalSSZ(buf)
}

// SizeSSZ returns the ssz encoded size in bytes for the Uint256 object
func (u *Uint256) SizeSSZ() (size int) {
	return u.Int.SizeSSZ()
}

// HashTreeRoot ssz hashes the Uint256 object
func (u *Uint256) HashTreeRoot() ([32]byte, error) {
	b, _ := u.MarshalSSZTo(make([]byte, 0, 32)) // ignore error, cannot fail
	var hash [32]byte
	copy(hash[:], b)
	return hash, nil
}

// HashTreeRootWith ssz hashes the Uint256 object with a hasher
func (u *Uint256) HashTreeRootWith(hh ssz.HashWalker) (err error) {
	bytes, _ := u.MarshalSSZTo(make([]byte, 0, 32)) // ignore error, cannot fail
	hh.AppendBytes32(bytes)
	return
}

// GetTree ssz hashes the Uint256 object
func (u *Uint256) GetTree() (*ssz.Node, error) {
	return ssz.ProofTree(u)
}

// MarshalBinary
func (u *Uint256) MarshalBinary() (data []byte, err error) {
	return u.Int.MarshalSSZ()
}

// Valuer
func (u Uint256) Value() (driver.Value, error) {
	return u.Int.ToBig(), nil
}
