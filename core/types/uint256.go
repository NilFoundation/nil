package types

import (
	ssz "github.com/ferranbt/fastssz"
	"github.com/holiman/uint256"
)

// interfaces
var _ ssz.Marshaler = (*Uint256)(nil)
var _ ssz.Unmarshaler = (*Uint256)(nil)
var _ ssz.HashRoot = (*Uint256)(nil)

type Uint256 struct{ uint256.Int }

// MarshalSSZ ssz marshals the Uint256 object
func (u *Uint256) MarshalSSZ() ([]byte, error) {
	return ssz.MarshalSSZ(u)
}

// MarshalSSZTo ssz marshals the Uint256 object to a target array
func (u *Uint256) MarshalSSZTo(buf []byte) (dst []byte, err error) {
	dst = buf
	if buf, err = u.Int.MarshalSSZ(); err != nil {
		return
	}
	dst = append(dst, buf...)

	return
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
	return ssz.HashWithDefaultHasher(u)
}

// HashTreeRootWith ssz hashes the Uint256 object with a hasher
func (u *Uint256) HashTreeRootWith(hh ssz.HashWalker) (err error) {
	indx := hh.Index()

	{
		subIndx := hh.Index()
		for _, i := range u.Int {
			hh.AppendUint64(i)
		}
		hh.Merkleize(subIndx)
	}

	hh.Merkleize(indx)
	return
}

// GetTree ssz hashes the Uint256 object
func (u *Uint256) GetTree() (*ssz.Node, error) {
	return ssz.ProofTree(u)
}
