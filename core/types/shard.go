package types

import (
	common "github.com/NilFoundation/nil/common"
	ssz "github.com/NilFoundation/nil/core/ssz"
	"github.com/rs/zerolog/log"
	"strconv"
)

type ShardId uint64

const MasterShardId = ShardId(0)

// interfaces
var _ ssz.SizedObjectSSZ = new(ShardId)
var _ common.Hashable = new(ShardId)

func (s ShardId) EncodeSSZ(dst []byte) ([]byte, error) {
	return ssz.MarshalSSZ(
		dst,
		ssz.Uint64SSZ(uint64(s)),
	)
}

func (s ShardId) EncodingSizeSSZ() int {
	return common.Uint64Size
}

func (s ShardId) Clone() common.Clonable {
	clonned := s
	return &clonned
}

func (s ShardId) DecodeSSZ(buf []byte, version int) error {
	err := ssz.UnmarshalSSZ(
		buf,
		0,
		(*uint64)(&s),
	)

	if err != nil {
		return err
	}
	return nil
}

func (s ShardId) Hash() common.Hash {
	h, err := ssz.SSZHash(s)
	if err != nil {
		log.Fatal().Msg(err.Error())
	}
	return h
}

func (s ShardId) Static() bool {
	return true
}

func (s ShardId) String() string { return strconv.FormatUint(uint64(s), 10) }
func (s ShardId) Bytes() []byte  { return []byte(s.String()) }
