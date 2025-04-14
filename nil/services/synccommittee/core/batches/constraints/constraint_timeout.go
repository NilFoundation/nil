package constraints

import (
	"context"

	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	"github.com/jonboulle/clockwork"
)

type timeoutConstraint struct {
	constraint BatchConstraints
	clock      clockwork.Clock
}

func newTimeoutConstraint(constraint BatchConstraints, clock clockwork.Clock) batchConstraintRunner {
	return &timeoutConstraint{
		constraint: constraint,
		clock:      clock,
	}
}

func (c *timeoutConstraint) Name() string {
	return "timeout"
}

func (c *timeoutConstraint) Run(_ context.Context, batch *types.BlockBatch) (*CheckResult, error) {
	if batch.IsEmpty() {
		return canBeExtended(), nil
	}

	now := c.clock.Now()
	currentDuration := now.Sub(batch.CreatedAt)
	sealingTimeout := c.constraint.SealingTimeout

	if currentDuration >= sealingTimeout {
		return shouldBeSealed("sealing timeout is reached (%s >= %s)", currentDuration, sealingTimeout), nil
	}

	return canBeExtended(), nil
}
