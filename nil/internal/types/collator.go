package types

import "github.com/ethereum/go-ethereum/rlp"

// Neighbor describes collator's current position in a neighbor shard.
type Neighbor struct {
	ShardId ShardId `json:"shardId"`

	// next block and transaction to read
	BlockNumber      BlockNumber      `json:"blockNumber"`
	TransactionIndex TransactionIndex `json:"transactionIndex"`
}

type CollatorState struct {
	Neighbors []Neighbor `json:"neighbors"`
}

func (c *CollatorState) UnmarshalNil(buf []byte) error {
	return rlp.DecodeBytes(buf, c)
}

func (c CollatorState) MarshalNil() ([]byte, error) {
	return rlp.EncodeToBytes(&c)
}
