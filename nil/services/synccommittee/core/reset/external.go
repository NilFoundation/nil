package reset

import (
	"context"

	"github.com/NilFoundation/nil/nil/common"
)

type BlockFetcher interface {
	Pause(ctx context.Context) error
	Resume(ctx context.Context) error
}

type BlockResetter interface {
	ResetProgress(ctx context.Context, firstMainHashToPurge common.Hash) error
}

type Service interface {
	Stop() (stopped <-chan struct{})
}
