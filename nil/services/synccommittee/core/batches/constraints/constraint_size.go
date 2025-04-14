package constraints

import (
	"context"

	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
)

type sizeConstraint struct {
	constraints BatchConstraints
}

func newSizeConstraint(constraints BatchConstraints) batchConstraintRunner {
	return &sizeConstraint{
		constraints: constraints,
	}
}

func (c *sizeConstraint) Name() string {
	return "size"
}

func (c *sizeConstraint) Run(_ context.Context, batch *types.BlockBatch) (*CheckResult, error) {
	blocksCount := uint32(batch.Blocks.BlocksCount())
	maxBlockCount := c.constraints.MaxBlocksCount

	switch {
	case blocksCount > maxBlockCount:
		return shouldBeDiscarded("batch size exceeded MaxBlocksCount (%d > %d)", blocksCount, maxBlockCount), nil

	case blocksCount == maxBlockCount:
		return shouldBeSealed("batch size is equal to MaxBlocksCount value (%d)", blocksCount), nil

	default:
		return canBeExtended(), nil
	}
}
