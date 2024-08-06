package types

// Neighbor describes collator's current position in a neighbor shard.
type Neighbor struct {
	ShardId ShardId

	// next block and message to read
	BlockNumber  BlockNumber
	MessageIndex MessageIndex
}

type CollatorState struct {
	Neighbors []Neighbor `ssz-max:"10000"`
}
