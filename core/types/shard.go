package types

import (
	"encoding/json"
	"strconv"
)

// 32 bits are more than enough while avoiding problems with marshaling 64-bit values as numbers in JSON.
type ShardId uint32

const MasterShardId = ShardId(0)

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

func (s ShardId) Static() bool {
	return true
}

func (s ShardId) String() string { return strconv.FormatUint(uint64(s), 10) }
func (s ShardId) Bytes() []byte  { return []byte(s.String()) }
