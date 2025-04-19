package reset

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/NilFoundation/nil/nil/common/logging"
	scTypes "github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
)

const (
	componentsResumeDelay   = 10 * time.Minute
	componentsResumeTimeout = time.Minute
	gracefulShutdownTimeout = 5 * time.Minute
)

type PausableComponent interface {
	Pause(ctx context.Context) error
	Resume(ctx context.Context) error
	Name() string
}

type Service interface {
	Stop() (stopped <-chan struct{})
}

type StateResetLauncher struct {
	pausableComponents []PausableComponent
	resetter           *StateResetter
	service            Service
	logger             logging.Logger
	isRunning          atomic.Bool
}

func NewResetLauncher(
	resetter *StateResetter,
	service Service,
	logger logging.Logger,
) *StateResetLauncher {
	return &StateResetLauncher{
		resetter: resetter,
		service:  service,
		logger:   logger,
	}
}

func (l *StateResetLauncher) AddPausableComponent(pausableComponent ...PausableComponent) {
	l.pausableComponents = append(l.pausableComponents, pausableComponent...)
}

func (l *StateResetLauncher) LaunchPartialResetWithSuspension(
	ctx context.Context, failedBatchId scTypes.BatchId,
) error {
	return l.withExclusiveLock(func() error {
		return l.unsafeLaunchPartialResetWithSuspension(ctx, failedBatchId)
	})
}

func (l *StateResetLauncher) unsafeLaunchPartialResetWithSuspension(
	ctx context.Context,
	failedBatchId scTypes.BatchId,
) error {
	l.logger.Info().
		Stringer(logging.FieldBatchId, failedBatchId).
		Msg("Launching state partial reset process")

	if err := l.pauseComponents(ctx, nil); err != nil {
		return err
	}

	if err := l.resetter.ResetProgressPartial(ctx, failedBatchId); err != nil {
		l.onResetError(ctx, err, failedBatchId)
		return nil
	}

	l.logger.Info().
		Stringer(logging.FieldBatchId, failedBatchId).
		Msgf("State partial reset completed, block components will be resumed after %s", componentsResumeDelay)

	time.AfterFunc(componentsResumeDelay, func() {
		l.resumeComponents(ctx)
	})
	return nil
}

// LaunchResetToL1WithSuspension pauses all components, resets all batches by removing them from storage,
// and calls `SetProvedStateRoot` using the root obtained from L1.
// The `caller` argument is used to exclude the caller from being paused;
// otherwise, `StateResetLauncher` will attempt to pause the caller as well, which will cause a `pauseComponents` error.
func (l *StateResetLauncher) LaunchResetToL1WithSuspension(
	ctx context.Context, caller PausableComponent,
) error {
	return l.withExclusiveLock(func() error {
		return l.unsafeLaunchResetToL1WithSuspension(ctx, caller)
	})
}

func (l *StateResetLauncher) unsafeLaunchResetToL1WithSuspension(
	ctx context.Context, caller PausableComponent,
) error {
	l.logger.Info().
		Msg("Launching state full reset process")

	if err := l.pauseComponents(ctx, caller); err != nil {
		l.logger.Error().Err(err).Msg("Pausing components failed")
		return err
	}

	if err := l.resetter.ResetProgressToL1(ctx); err != nil {
		l.logger.Error().Err(err).Msg("Failed to reset all progress")
		l.resumeComponents(ctx)
		return nil
	}

	l.logger.Info().
		Msgf("State full reset completed, block components will be resumed after %s", componentsResumeDelay)

	time.AfterFunc(componentsResumeDelay, func() {
		l.resumeComponents(ctx)
	})
	return nil
}

func (l *StateResetLauncher) onResetError(
	ctx context.Context, resetErr error, failedBatchId scTypes.BatchId,
) {
	l.logger.Error().Err(resetErr).Stringer(logging.FieldBatchId, failedBatchId).Msg("Failed to reset progress")
	l.resumeComponents(ctx)
}

func (l *StateResetLauncher) pauseComponents(ctx context.Context, skipComponent PausableComponent) error {
	l.logger.Info().Msg("Pausing components")
	for _, component := range l.pausableComponents {
		if component == skipComponent {
			continue
		}
		if err := component.Pause(ctx); err != nil {
			return fmt.Errorf("failed to pause component %s: %w", component.Name(), err)
		}
	}
	return nil
}

func (l *StateResetLauncher) resumeComponents(ctx context.Context) {
	l.logger.Info().Msg("Resuming components")

	detachedCtx := context.WithoutCancel(ctx)
	var resumeErr error
	for _, component := range l.pausableComponents {
		timeoutCtx, cancel := context.WithTimeout(detachedCtx, componentsResumeTimeout)

		err := component.Resume(timeoutCtx)
		cancel()

		if err != nil {
			resumeErr = fmt.Errorf("failed to resume component %s: %w", component.Name(), err)
			break
		}
	}

	if resumeErr == nil {
		l.logger.Info().Msg("Block fetching successfully resumed")
		return
	}

	l.logger.Error().Err(resumeErr).Msg("Failed to resume block fetching, service will be terminated")

	stopped := l.service.Stop()

	select {
	case <-time.After(gracefulShutdownTimeout):
		l.logger.Fatal().Err(resumeErr).
			Msgf("Service did not stop after %s, force termination", gracefulShutdownTimeout)
	case <-stopped:
	}
}

func (l *StateResetLauncher) withExclusiveLock(f func() error) error {
	if !l.isRunning.CompareAndSwap(false, true) {
		l.logger.Warn().Msg("Reset already in progress, ignoring duplicate call")
		return errors.New("reset already in progress")
	}
	defer l.isRunning.Store(false)

	return f()
}
