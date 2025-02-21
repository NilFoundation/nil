package reset

import (
	"context"
	"fmt"
	"time"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/rs/zerolog"
)

const (
	fetchResumeDelay        = 10 * time.Minute
	fetchResumeTimeout      = time.Minute
	gracefulShutdownTimeout = 5 * time.Minute
)

type stateResetLauncher struct {
	blockFetcher  BlockFetcher
	blockResetter BlockResetter
	service       Service
	logger        zerolog.Logger
}

func NewResetLauncher(
	blockFetcher BlockFetcher,
	blockResetter BlockResetter,
	service Service,
	logger zerolog.Logger,
) *stateResetLauncher {
	return &stateResetLauncher{
		blockFetcher:  blockFetcher,
		blockResetter: blockResetter,
		service:       service,
		logger:        logger,
	}
}

func (l *stateResetLauncher) ResetState(ctx context.Context, failedMainBlockHash common.Hash) error {
	l.logger.Info().Msg("Starting state reset process")

	if err := l.blockFetcher.Pause(ctx); err != nil {
		return fmt.Errorf("failed to pause block fetching: %w", err)
	}

	if err := l.blockResetter.ResetProgress(ctx, failedMainBlockHash); err != nil {
		resetErr := fmt.Errorf("failed to reset blocks progress: %w", err)
		l.onResetError(resetErr, failedMainBlockHash)
		return resetErr
	}

	l.logger.Info().Msgf("State reset completed, block fetching will be resumed after %s", fetchResumeDelay)
	time.AfterFunc(fetchResumeDelay, l.resumeBlockFetching)
	return nil
}

func (l *stateResetLauncher) onResetError(resetErr error, failedMainBlockHash common.Hash) {
	l.logger.Error().Err(resetErr).Stringer(logging.FieldBlockMainChainHash, failedMainBlockHash).Send()
	l.resumeBlockFetching()
}

func (l *stateResetLauncher) resumeBlockFetching() {
	ctx, cancel := context.WithTimeout(context.Background(), fetchResumeTimeout)
	defer cancel()

	l.logger.Info().Msg("Resuming block fetching")
	err := l.blockFetcher.Resume(ctx)

	if err == nil {
		l.logger.Info().Msg("Block fetching successfully resumed")
		return
	}

	l.logger.Error().Err(err).Msg("Failed to resume block fetching, service will be terminated")

	stopped := l.service.Stop()

	select {
	case <-time.After(gracefulShutdownTimeout):
		l.logger.Fatal().Err(err).Msgf("Service did not stop after %s, force termination", gracefulShutdownTimeout)
	case <-stopped:
	}
}
