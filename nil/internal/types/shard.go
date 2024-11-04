package types

import (
	"encoding/binary"
	"encoding/json"
	"math"
	"strconv"
)

// 32 bits are more than enough while avoiding problems with marshaling 64-bit values as numbers in JSON.
type ShardId uint32

const (
	MainShardId    = ShardId(0)
	BaseShardId    = ShardId(1)
	InvalidShardId = ShardId(math.MaxUint32)
)

func (s ShardId) IsMainShard() bool {
	return s == MainShardId
}

func (s ShardId) MarshalJSON() ([]byte, error) {
	return json.Marshal(uint32(s))
}

func (s *ShardId) UnmarshalJSON(data []byte) error {
	var id uint32
	if err := json.Unmarshal(data, &id); err != nil {
		return err
	}
	*s = ShardId(id)
	return nil
}

func (s *ShardId) Set(val string) error {
	var err error
	*s, err = ParseShardIdFromString(val)
	return err
}

func (s *ShardId) Type() string {
	return "ShardId"
}

func NewShardId(value *ShardId, defaultValue ShardId) *ShardId {
	*value = defaultValue
	return value
}

func (s ShardId) Static() bool {
	return true
}

func BytesToShardId(b []byte) ShardId {
	return ShardId(binary.BigEndian.Uint16(b))
}

func ParseShardIdFromString(s string) (ShardId, error) {
	id, err := strconv.ParseInt(s, 10, 32)
	if err != nil {
		return 0, err
	}
	return ShardId(id), nil
}

func (s ShardId) String() string { return strconv.FormatUint(uint64(s), 10) }
func (s ShardId) Bytes() []byte {
	bytes := make([]byte, 2)
	binary.BigEndian.PutUint16(bytes, uint16(s))
	return bytes
}
