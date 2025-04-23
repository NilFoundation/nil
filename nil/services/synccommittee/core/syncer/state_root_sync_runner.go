package syncer

import (
	"context"
	"fmt"
	"time"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/srv"
)

// stateRootSyncRunner is responsible for synchronizing the local state root with the
// latest finalized state root from L1.
//
// The runner will retry syncing the state root during application startup
// until it succeeds or the context is canceled.
//
// Once the runner successfully syncs the state root, it sends a
// message on the started channel and exits, letting the following workers to start.
type stateRootSyncRunner struct {
	syncer  *stateRootSyncer
	retrier common.RetryRunner
	logger  logging.Logger
}

func NewRunner(
	syncer *stateRootSyncer,
	logger logging.Logger,
) *stateRootSyncRunner {
	check.PanicIff(syncer == nil, "syncer is nil")

	retrier := common.NewRetryRunner(
		common.RetryConfig{
			ShouldRetry: common.DoNotRetryIf(context.Canceled, context.DeadlineExceeded),
			NextDelay:   common.DelayExponential(time.Second, time.Minute),
		},
		logger,
	)

	runner := &stateRootSyncRunner{
		syncer:  syncer,
		retrier: retrier,
	}

	runner.logger = srv.WorkerLogger(logger, runner)
	return runner
}

func (s *stateRootSyncRunner) Name() string {
	return "state_root_syncer"
}

func (s *stateRootSyncRunner) Run(ctx context.Context, started chan<- struct{}) error {
	s.logger.Info().Msg("Syncing state with L1")

	err := s.retrier.Do(ctx, func(ctx context.Context) error {
		return s.syncer.SyncLatestFinalizedRoot(ctx)
	})
	if err != nil {
		return fmt.Errorf("failed to sync L1 state root: %w", err)
	}

	started <- struct{}{}
	return nil
}
