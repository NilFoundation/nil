package shardchain

import (
	"context"

	"github.com/NilFoundation/nil/core/execution"
)

func GenerateZeroState(ctx context.Context, es *execution.ExecutionState) error {
	return es.GenerateZeroState(ctx)
}
