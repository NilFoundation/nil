package types

import (
	common "github.com/NilFoundation/nil/common"
	ssz "github.com/NilFoundation/nil/core/ssz"
	"github.com/holiman/uint256"
	"log"
)

type SmartContract struct {
	Address     common.Address
	Initialised bool
	Balance     uint256.Int
	StorageRoot common.Hash
	CodeHash    common.Hash
}

func (s *SmartContract) EncodeSSZ(dst []byte) ([]byte, error) {
	return ssz.MarshalSSZ(
		dst,
		s.Address[:],
		ssz.BoolSSZ(s.Initialised),
		ssz.Uint256SSZ(s.Balance),
		s.StorageRoot[:],
		s.CodeHash[:],
	)
}

func (s *SmartContract) DecodeSSZ(buf []byte, version int) error {
	balanceBytes := make([]byte, 32)
	err := ssz.UnmarshalSSZ(
		buf,
		0,
		&s.Address,
		&s.Initialised,
		&balanceBytes,
		&s.StorageRoot,
		&s.CodeHash,
	)

	if err != nil {
		return err
	}

	s.Balance.SetBytes(balanceBytes)
	return nil
}

func (s *SmartContract) EncodingSizeSSZ() int {
	return common.Bytes64Size + common.HashSize + common.HashSize
}

func (s *SmartContract) Hash() common.Hash {
	h, err := ssz.SSZHash(s)
	if err != nil {
		log.Fatal(err)
	}
	return h
}
