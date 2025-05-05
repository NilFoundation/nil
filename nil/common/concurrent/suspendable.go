package concurrent

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"
)

type workerState int8

const (
	_ workerState = iota
	workerStateRunning
	workerStatePaused
)

var ErrSuspendableTerminated = errors.New("suspendable action was terminated")

type stateChangeRequest struct {
	newState workerState
	response chan bool
}

// Suspendable provides a mechanism for suspending and resuming periodic execution of an action.
type Suspendable struct {
	action    func(context.Context) error
	interval  time.Duration
	stateCh   chan stateChangeRequest
	stoppedCh chan error
}

func NewSuspendable(action func(context.Context) error, interval time.Duration) Suspendable {
	return Suspendable{
		action:    action,
		interval:  interval,
		stateCh:   make(chan stateChangeRequest),
		stoppedCh: make(chan error, 1),
	}
}

// Run executes a suspendable action periodically based on the provided interval until the context is canceled.
// It listens for pause and resume signals, halting and resuming execution accordingly.
func (s *Suspendable) Run(ctx context.Context, started chan<- struct{}) error {
	defer close(s.stoppedCh)

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	if started != nil {
		close(started)
	}

	state := workerStateRunning

	for {
		select {
		case <-ctx.Done():
			err := ctx.Err()
			s.stoppedCh <- err
			return err

		case <-ticker.C:
			if err := s.action(ctx); err != nil {
				s.stoppedCh <- err
				return err
			}

		case req := <-s.stateCh:
			s.onStateChange(ticker, &state, req)
		}
	}
}

func (s *Suspendable) onStateChange(ticker *time.Ticker, currentState *workerState, req stateChangeRequest) {
	defer close(req.response)

	switch req.newState {
	case *currentState:
		// state remains unchanged, push false to the caller of Pause() / Resume()
		req.response <- false
		return

	case workerStatePaused:
		ticker.Stop()

	case workerStateRunning:
		ticker.Reset(s.interval)

	default:
		log.Panicf("unknown worker state: %d", req.newState)
	}

	*currentState = req.newState
	req.response <- true
}

// Pause halts the periodic execution of the action, transitioning the worker to a paused state.
// Returns true if the state was changed, false if already paused or an error occurred.
func (s *Suspendable) Pause(ctx context.Context) (paused bool, err error) {
	return s.pushAndWait(ctx, workerStatePaused)
}

// Resume resumes periodic action execution, transitioning the worker to a running state,
// Returns true if the state was changed, false if already running or an error occurred.
func (s *Suspendable) Resume(ctx context.Context) (resumed bool, err error) {
	return s.pushAndWait(ctx, workerStateRunning)
}

func (s *Suspendable) pushAndWait(ctx context.Context, newState workerState) (bool, error) {
	request := stateChangeRequest{newState: newState, response: make(chan bool)}

	select {
	case <-ctx.Done():
		return false, ctx.Err()

	case s.stateCh <- request:
		select {
		case <-ctx.Done():
			return false, ctx.Err()

		case stateWasChanged := <-request.response:
			return stateWasChanged, nil
		}

	case err := <-s.stoppedCh:
		return false, fmt.Errorf("%w: %w", ErrSuspendableTerminated, err)
	}
}
