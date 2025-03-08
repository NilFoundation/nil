package concurrent

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"golang.org/x/sync/errgroup"
)

const (
	testTimeout        = 10 * time.Second
	testActionInterval = 10 * time.Millisecond
)

type SuspendableTestSuite struct {
	suite.Suite

	ctx    context.Context
	cancel context.CancelFunc
}

func TestSuspendableTestSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(SuspendableTestSuite))
}

func (s *SuspendableTestSuite) SetupTest() {
	s.ctx, s.cancel = context.WithTimeout(context.Background(), testTimeout)
}

func (s *SuspendableTestSuite) TearDownTest() {
	s.cancel()
}

func (s *SuspendableTestSuite) Test_Run_Simple() {
	suspendable := s.newRunningSuspendable(s.ctx)
	s.requireHasCalls(suspendable)
}

func (s *SuspendableTestSuite) Test_Run_Cancelled() {
	ctx, cancel := context.WithCancel(s.ctx)
	defer cancel()

	suspendable := NewSuspendable(s.noopAction(), testActionInterval)

	var errGroup errgroup.Group

	errGroup.Go(func() error {
		return suspendable.Run(ctx, nil)
	})

	cancel()
	err := errGroup.Wait()
	s.Require().ErrorIs(err, context.Canceled)
}

func (s *SuspendableTestSuite) Test_Pause_Successfully() {
	suspendable := s.newRunningSuspendable(s.ctx)
	s.requireHasCalls(suspendable)

	paused, err := suspendable.Pause(s.ctx)
	s.Require().NoError(err)
	s.Require().True(paused)

	callsAfterPause := suspendable.numOfCalls.Load()

	// no new calls once suspendable is paused
	s.Require().Never(func() bool {
		return suspendable.numOfCalls.Load() > callsAfterPause
	}, 200*time.Millisecond, 20*time.Millisecond)
}

func (s *SuspendableTestSuite) Test_Pause_Not_Running_Timeout() {
	suspendable := NewSuspendable(s.noopAction(), testActionInterval)

	ctx, cancel := context.WithTimeout(s.ctx, 100*time.Millisecond)
	defer cancel()

	paused, err := suspendable.Pause(ctx)
	s.Require().ErrorIs(err, context.DeadlineExceeded)
	s.Require().False(paused)
}

func (s *SuspendableTestSuite) Test_Pause_Long_Running_Action() {
	var called atomic.Bool
	action := func(ctx context.Context) {
		called.Store(true)
		time.Sleep(500 * time.Millisecond)
	}
	suspendable := NewSuspendable(action, testActionInterval)

	go func() {
		_ = suspendable.Run(s.ctx, nil)
	}()

	s.Require().Eventually(called.Load, 3*time.Second, 10*time.Millisecond)

	paused, err := suspendable.Pause(s.ctx)
	s.Require().NoError(err)
	s.Require().True(paused)
}

func (s *SuspendableTestSuite) Test_Pause_N_Times() {
	suspendable := s.newRunningSuspendable(s.ctx)

	s.Require().Eventually(func() bool {
		paused, err := suspendable.Pause(s.ctx)
		return paused && err == nil
	}, 3*time.Second, 10*time.Millisecond)

	for range 3 {
		paused, err := suspendable.Pause(s.ctx)
		s.Require().NoError(err)
		s.Require().False(paused)
	}
}

func (s *SuspendableTestSuite) Test_Pause_Cancel() {
	suspendable := s.newRunningSuspendable(s.ctx)
	s.requireHasCalls(suspendable)

	ctx, cancel := context.WithCancel(s.ctx)
	cancel()
	paused, err := suspendable.Pause(ctx)

	s.Require().False(paused)
	s.Require().ErrorIs(err, context.Canceled)
}

func (s *SuspendableTestSuite) Test_Pause_Worker_Cancelled() {
	ctx, cancel := context.WithCancel(s.ctx)
	defer cancel()

	suspendable := s.newRunningSuspendable(ctx)
	s.requireHasCalls(suspendable)

	cancel()

	paused, err := suspendable.Pause(s.ctx)
	s.Require().ErrorIs(err, ErrWorkerStopped)
	s.Require().False(paused)
}

func (s *SuspendableTestSuite) Test_Cancel_After_Paused() {
	ctx, cancel := context.WithCancel(s.ctx)
	defer cancel()

	suspendable := NewSuspendable(s.noopAction(), testActionInterval)

	var errGroup errgroup.Group
	errGroup.Go(func() error {
		return suspendable.Run(ctx, nil)
	})

	s.Require().Eventually(func() bool {
		paused, err := suspendable.Pause(s.ctx)
		return paused && err == nil
	}, 3*time.Second, 10*time.Millisecond)

	cancel()
	err := errGroup.Wait()
	s.Require().ErrorIs(err, context.Canceled)
}

func (s *SuspendableTestSuite) Test_Pause_Resume_N_Times() {
	suspendable := s.newRunningSuspendable(s.ctx)
	s.requireHasCalls(suspendable)

	for range 3 {
		paused, err := suspendable.Pause(s.ctx)
		s.Require().NoError(err)
		s.Require().True(paused)

		callsBeforeResume := suspendable.numOfCalls.Load()

		resumed, err := suspendable.Resume(s.ctx)
		s.Require().NoError(err)
		s.Require().True(resumed)

		s.Require().Eventually(func() bool {
			callsAfterResume := suspendable.numOfCalls.Load()
			return callsAfterResume > callsBeforeResume
		}, 3*time.Second, 10*time.Millisecond)
	}
}

func (s *SuspendableTestSuite) Test_Resume_Not_Running_Timeout() {
	suspendable := NewSuspendable(s.noopAction(), testActionInterval)

	ctx, cancel := context.WithTimeout(s.ctx, 100*time.Millisecond)
	defer cancel()

	resumed, err := suspendable.Resume(ctx)
	s.Require().ErrorIs(err, context.DeadlineExceeded)
	s.Require().False(resumed)
}

func (s *SuspendableTestSuite) Test_Resume_N_Times() {
	suspendable := s.newRunningSuspendable(s.ctx)

	s.Require().Eventually(func() bool {
		paused, err := suspendable.Pause(s.ctx)
		return paused && err == nil
	}, 3*time.Second, 10*time.Millisecond)

	resumed, err := suspendable.Resume(s.ctx)
	s.Require().NoError(err)
	s.Require().True(resumed)

	for range 3 {
		resumed, err := suspendable.Resume(s.ctx)
		s.Require().NoError(err)
		s.Require().False(resumed)
	}
}

func (s *SuspendableTestSuite) Test_Resume_Cancel() {
	suspendable := s.newRunningSuspendable(s.ctx)

	s.Require().Eventually(func() bool {
		paused, err := suspendable.Pause(s.ctx)
		return paused && err == nil
	}, 3*time.Second, 10*time.Millisecond)

	ctx, cancel := context.WithCancel(s.ctx)
	cancel()

	resumed, err := suspendable.Resume(ctx)
	s.Require().False(resumed)
	s.Require().ErrorIs(err, context.Canceled)
}

func (s *SuspendableTestSuite) Test_Resume_Worker_Cancelled() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	suspendable := s.newRunningSuspendable(ctx)
	s.requireHasCalls(suspendable)

	cancel()

	resumed, err := suspendable.Resume(s.ctx)
	s.Require().ErrorIs(err, ErrWorkerStopped)
	s.Require().False(resumed)
}

func (s *SuspendableTestSuite) noopAction() func(ctx context.Context) {
	s.T().Helper()
	return func(ctx context.Context) {}
}

type testSuspendable struct {
	*Suspendable
	numOfCalls *atomic.Int32
}

func (s *SuspendableTestSuite) newRunningSuspendable(ctx context.Context) testSuspendable {
	s.T().Helper()

	var numOfCalls atomic.Int32
	action := func(ctx context.Context) {
		numOfCalls.Add(1)
	}
	suspendable := NewSuspendable(action, testActionInterval)

	go func() {
		_ = suspendable.Run(ctx, nil)
	}()

	return testSuspendable{suspendable, &numOfCalls}
}

func (s *SuspendableTestSuite) requireHasCalls(suspendable testSuspendable) {
	s.T().Helper()
	s.Require().Eventually(func() bool {
		return suspendable.numOfCalls.Load() >= 3
	}, 300*time.Second, 10*time.Millisecond)
}
