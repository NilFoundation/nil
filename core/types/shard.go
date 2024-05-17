package types

import (
	"strconv"
)

type ShardId uint64

const MasterShardId = ShardId(0)

func (s ShardId) Static() bool {
	return true
}

func (s ShardId) String() string { return strconv.FormatUint(uint64(s), 10) }
func (s ShardId) Bytes() []byte  { return []byte(s.String()) }
