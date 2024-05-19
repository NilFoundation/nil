package types

import (
	"encoding/json"
	"strconv"
)

type ShardId uint64

const MasterShardId = ShardId(0)

func (s ShardId) MarshalJSON() ([]byte, error) {
	return json.Marshal(uint64(s))
}

func (s *ShardId) UnmarshalJSON(data []byte) error {
	var id uint64
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
