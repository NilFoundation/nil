package concurrent

import (
	"context"
	"time"
)

type workerState int32

const (
	_ workerState = iota
	workerStateRunning
	workerStatePaused
)

type stateChangeRequest struct {
	newState workerState
	response chan bool
}

// Suspendable provides a mechanism for suspending and resuming periodic execution of an action.
type Suspendable struct {
	action   func(context.Context)
	interval time.Duration
	stateCh  chan stateChangeRequest
}

func NewSuspendable(action func(context.Context), interval time.Duration) *Suspendable {
	return &Suspendable{
		action:   action,
		interval: interval,
		stateCh:  make(chan stateChangeRequest),
	}
}

// Run executes a suspendable action periodically based on the provided interval until the context is canceled.
// It listens for pause and resume signals, halting and resuming execution accordingly.
func (s *Suspendable) Run(ctx context.Context) error {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	state := workerStateRunning

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-ticker.C:
			if state == workerStateRunning {
				s.action(ctx)
			}

		case req := <-s.stateCh:
			if req.newState == state {
				// state remains unchanged, push false to the caller of Pause() / Resume()
				req.response <- false
				continue
			}

			state = req.newState
			req.response <- true
		}
	}
}

func (s *Suspendable) Pause(ctx context.Context) (paused bool, err error) {
	return s.pushAndWait(ctx, workerStatePaused)
}

func (s *Suspendable) Resume(ctx context.Context) (resumed bool, err error) {
	return s.pushAndWait(ctx, workerStateRunning)
}

func (s *Suspendable) pushAndWait(ctx context.Context, newState workerState) (bool, error) {
	request := stateChangeRequest{newState: newState, response: make(chan bool, 1)}

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

	default:
		return false, nil
	}
}
