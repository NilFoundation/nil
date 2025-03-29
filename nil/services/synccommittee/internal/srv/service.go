package srv

import (
	"context"
	"errors"
	"fmt"
	"os/signal"
	"runtime/debug"
	"slices"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/NilFoundation/nil/nil/common/logging"
)

const (
	workerStopTimeout = 60 * time.Second
)

type Service struct {
	workers      []Worker
	started      atomic.Bool
	cancellation chan context.CancelFunc
	stopped      chan struct{}
	logger       logging.Logger
}

func NewService(logger logging.Logger, workers ...Worker) Service {
	return Service{
		workers:      workers,
		cancellation: make(chan context.CancelFunc, 1),
		stopped:      make(chan struct{}),
		logger:       logger,
	}
}

type workerControl struct {
	cancel  context.CancelFunc
	started chan struct{}
	stopped chan error
}

// Run executes all workers concurrently, listens for termination signals, and handles graceful shutdown procedures.
// Service's workers are started in forward order and stopped in reverse order.
// For example, given workers [A, B, C, D]:
//
// When the Run method is called, workers are started in the order: A -> B -> C -> D.
//
// If worker C fails to start, the workers that were already started (B and A) will
// be stopped, and worker D will not be executed at all.
//
// On receiving a termination signal (e.g., SIGINT or SIGTERM), the shutdown process begins.
// During shutdown, workers are stopped in the reverse order: D -> C -> B -> A.
func (s *Service) Run(ctx context.Context) error {
	if !s.started.CompareAndSwap(false, true) {
		return errors.New("service already started")
	}

	s.logger.Info().Msg("starting service")

	signalCtx, cancellation := signal.NotifyContext(ctx, syscall.SIGTERM, syscall.SIGINT)
	s.cancellation <- cancellation
	close(s.cancellation)

	defer func() {
		s.tryCancel()
		close(s.stopped)
	}()

	controls := make([]*workerControl, len(s.workers))
	defer s.cancelWorkers(controls)

	workerErrors := make(chan error, len(s.workers))

	for i, worker := range s.workers {
		control, shouldContinue := s.runWorker(ctx, workerErrors, worker)
		controls[i] = control
		if !shouldContinue {
			break
		}
	}

	select {
	case workerErr := <-workerErrors:
		s.logger.Error().Err(workerErr).Msg("stopping service due to a worker error")
		return workerErr
	case <-signalCtx.Done():
		s.logger.Info().Msg("stopping service due to a cancellation request")
		return signalCtx.Err()
	}
}

func (s *Service) Stop() (stopped <-chan struct{}) {
	s.tryCancel()
	return s.stopped
}

func (s *Service) tryCancel() {
	if cancel, ok := <-s.cancellation; ok {
		cancel()
	} else {
		s.logger.Debug().Msg("service execution already cancelled, ignoring cancellation request")
	}
}

func (s *Service) runWorker(
	ctx context.Context,
	workerErrors chan<- error,
	worker Worker,
) (control *workerControl, shouldContinue bool) {
	// Detach from the parent context's cancellation signal.
	// Cancellation is triggered sequentially per worker in the `cancelWorkers()` method.
	wCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	control = &workerControl{cancel, make(chan struct{}, 1), make(chan error)}

	go func(wCtx context.Context) {
		s.runWorkerWithControl(wCtx, workerErrors, worker, control)
	}(wCtx)

	logger := WorkerLogger(s.logger, worker)

	select {
	case <-ctx.Done():
		return control, false

	case <-control.started:
		logger.Info().Msg("worker started")
		return control, true

	case err := <-control.stopped:
		cancel()
		if err != nil {
			return nil, false
		}
		logger.Info().Msg("worker stopped")
		return nil, true
	}
}

func (s *Service) runWorkerWithControl(
	ctx context.Context,
	workerErrors chan<- error,
	worker Worker,
	control *workerControl,
) {
	logger := WorkerLogger(s.logger, worker)

	defer close(control.stopped)

	defer func() {
		r := recover()
		if r == nil {
			return
		}
		logger.Error().Interface("panic", r).Str("stacktrace", string(debug.Stack())).Msg("worker panicked")
		err := fmt.Errorf("worker %s panicked: %v", worker.Name(), r)
		workerErrors <- err
		control.stopped <- err
	}()

	logger.Info().Msg("starting worker")
	err := worker.Run(ctx, control.started)

	if nilOrCancelled(err) {
		return
	}

	logger.Error().Err(err).Msg("error running worker")
	workerErrors <- fmt.Errorf("error running worker %s: %w", worker.Name(), err)
	control.stopped <- err
}

func (s *Service) cancelWorkers(controls []*workerControl) {
	s.logger.Info().Msg("stopping all workers")

	for i, worker := range slices.Backward(s.workers) {
		control := controls[i]
		if control == nil {
			continue
		}
		logger := WorkerLogger(s.logger, worker)
		logger.Info().Msg("stopping worker")

		control.cancel()

		select {
		case <-time.After(workerStopTimeout):
			logger.Warn().Msg("worker did not stop in time")
		case err := <-control.stopped:
			if nilOrCancelled(err) {
				logger.Info().Msg("worker stopped")
			} else {
				logger.Error().Err(err).Msg("error stopping worker")
			}
		}
	}

	s.logger.Info().Msg("all workers stopped")
}

func nilOrCancelled(err error) bool {
	return err == nil || errors.Is(err, context.Canceled)
}
