package types

// Neighbor describes collator's current position in a neighbor shard.
type Neighbor struct {
	ShardId ShardId `json:"shardId"`

	// next block and transaction to read
	BlockNumber      BlockNumber      `json:"blockNumber"`
	TransactionIndex TransactionIndex `json:"transactionIndex"`
}

type CollatorState struct {
	Neighbors []Neighbor `json:"neighbors" ssz-max:"10000"`
}

func (c *CollatorState) UnmarshalNil(buf []byte) error {
	return c.UnmarshalSSZ(buf)
}

func (c CollatorState) MarshalNil() ([]byte, error) {
	return c.MarshalSSZ()
}
