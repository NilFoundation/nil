package constraints

import (
	"context"
	"fmt"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	"github.com/jonboulle/clockwork"
)

type batchConstraintRunner interface {
	Name() string
	Run(ctx context.Context, batch *types.BlockBatch) (*CheckResult, error)
}

type checker struct {
	constraints BatchConstraints
	runners     []batchConstraintRunner
	logger      logging.Logger
}

func NewChecker(
	constraints BatchConstraints,
	clock clockwork.Clock,
	logger logging.Logger,
) *checker {
	return &checker{
		constraints: constraints,
		runners: []batchConstraintRunner{
			newTimeoutConstraint(constraints, clock),
			newSizeConstraint(constraints),
		},
		logger: logger,
	}
}

func (c *checker) Constraints() BatchConstraints {
	return c.constraints
}

func (c *checker) CheckConstraints(ctx context.Context, batch *types.BlockBatch) (*CheckResult, error) {
	c.logger.Debug().Stringer(logging.FieldBatchId, batch.Id).Msg("Checking batch constraints")

	batchResult := canBeExtended()

	for _, constraint := range c.runners {
		result, err := constraint.Run(ctx, batch)
		if err != nil {
			return nil, fmt.Errorf("failed to run batch constraint %s: %w", constraint.Name(), err)
		}
		batchResult.JoinWith(result)
	}

	if batchResult.Type == CheckResultTypeCanBeExtended {
		c.logger.Debug().Stringer(logging.FieldBatchId, batch.Id).Msg("All batch constraints are satisfied")
	} else {
		c.logger.Info().
			Stringer(logging.FieldBatchId, batch.Id).
			Msgf("Batch constraint(s) are violated, result: %s", batchResult)
	}

	return batchResult, nil
}
