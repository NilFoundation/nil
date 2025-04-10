package events

import (
	"context"

	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
)

type BatchStatusChangeHandler interface {
	SupportedStatuses() map[types.BatchStatus]struct{}
	HandleStateChange(ctx context.Context, batch *types.BlockBatch) (*types.BlockBatch, error)
}
