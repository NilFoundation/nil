package types

import (
	common "github.com/NilFoundation/nil/common"
	ssz "github.com/NilFoundation/nil/core/ssz"
	"github.com/holiman/uint256"
	"github.com/rs/zerolog/log"
)

type SmartContract struct {
	Address     common.Address
	Initialised bool
	Balance     uint256.Int
	StorageRoot common.Hash
	CodeHash    common.Hash
}

// interfaces
var _ common.Hashable = new(SmartContract)
var _ ssz.SSZEncodable = new(SmartContract)
var _ ssz.SSZDecodable = new(SmartContract)

func (s *SmartContract) EncodeSSZ(dst *[]byte) error {
	return ssz.MarshalSSZ(
		dst,
		s.Address[:],
		ssz.BoolSSZ(s.Initialised),
		ssz.Uint256SSZ(s.Balance),
		s.StorageRoot[:],
		s.CodeHash[:],
	)
}

func (s *SmartContract) EncodingSizeSSZ() int {
	return common.AddrSize + common.BoolSize + common.Bits256Size + common.HashSize + common.HashSize
}

func (s *SmartContract) Clone() common.Clonable {
	clonned := *s
	return &clonned
}

func (s *SmartContract) DecodeSSZ(buf []byte, version int) error {
	balanceBytes := make([]byte, common.Bits256Size)
	err := ssz.UnmarshalSSZ(
		buf,
		0,
		s.Address[:],
		&s.Initialised,
		balanceBytes,
		s.StorageRoot[:],
		s.CodeHash[:],
	)

	if err != nil {
		return err
	}

	s.Balance.SetBytes(balanceBytes)
	return nil
}

func (s *SmartContract) Hash() common.Hash {
	h, err := ssz.SSZHash(s)
	if err != nil {
		log.Fatal().Msg(err.Error())
	}
	return h
}

func (s *SmartContract) Static() bool {
	return true
}
