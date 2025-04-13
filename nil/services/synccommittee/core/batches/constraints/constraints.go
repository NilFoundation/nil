package constraints

import "time"

type BatchConstraints struct {
	// SealingTimeout defines the max allowed interval between batch creation
	// and batch sealing (when batch is considered complete).
	SealingTimeout time.Duration

	// MaxBlocksCount specifies the maximum number of blocks allowed
	// to be included into a single batch.
	MaxBlocksCount uint32
}

func DefaultBatchConstraints() BatchConstraints {
	return BatchConstraints{
		SealingTimeout: 10 * time.Minute,
		MaxBlocksCount: 1000,
	}
}
