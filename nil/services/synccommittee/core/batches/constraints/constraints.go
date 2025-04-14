package constraints

import "time"

type BatchConstraints struct {
	// SealingTimeout defines the max allowed interval between batch creation
	// and batch sealing (when batch is considered complete).
	SealingTimeout time.Duration

	// MaxBlocksCount specifies the maximum number of blocks allowed
	// to be included in a single batch.
	MaxBlocksCount uint32
}

func NewBatchConstraints(
	sealingTimeout time.Duration,
	maxBlocksCount uint32,
) BatchConstraints {
	return BatchConstraints{
		SealingTimeout: sealingTimeout,
		MaxBlocksCount: maxBlocksCount,
	}
}

func DefaultBatchConstraints() BatchConstraints {
	const defaultSealingTimeout = 12 * time.Second
	const defaultMaxBlocksCount = 100
	return NewBatchConstraints(defaultSealingTimeout, defaultMaxBlocksCount)
}
