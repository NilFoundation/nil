package srv

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/stretchr/testify/suite"
)

const serviceTerminateTimeout = 10 * time.Second

type ServiceTestSuite struct {
	suite.Suite

	ctx          context.Context
	cancellation context.CancelFunc

	logger logging.Logger
}

func TestServiceTestSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(ServiceTestSuite))
}

func (s *ServiceTestSuite) SetupTest() {
	s.ctx, s.cancellation = context.WithCancel(context.Background())
	s.logger = logging.NewLogger("service_test")
}

func (s *ServiceTestSuite) TearDownTest() {
	s.cancellation()
}

type runTestCase struct {
	name        string
	workers     []Worker
	expectedErr error
}

func (s *ServiceTestSuite) Test_Run_With_Errors() {
	workerErr := errors.New("worker error")

	testCases := []runTestCase{
		{
			name: "Single_Worker_Error_After_Started",
			workers: []Worker{
				newWorkerMock("worker_0", func(ctx context.Context, started chan<- struct{}) error {
					close(started)
					return workerErr
				}),
			},
			expectedErr: workerErr,
		},
		{
			name: "Multiple_Workers_Mixed_Results",
			workers: []Worker{
				newWorkerMock("worker_0", func(ctx context.Context, started chan<- struct{}) error {
					close(started)
					return nil
				}),
				newWorkerMock("worker_1", func(ctx context.Context, startedCh chan<- struct{}) error {
					return workerErr
				}),
			},
			expectedErr: workerErr,
		},
	}

	for _, testCase := range testCases {
		s.Run(testCase.name, func() {
			errorCh := s.runInBackground(s.ctx, testCase.workers...)
			err := s.waitWithTimeout(s.ctx, errorCh)
			s.Require().ErrorIs(err, testCase.expectedErr)
		})
	}
}

func (s *ServiceTestSuite) Test_Run_Already_Cancelled() {
	newWorker := func() Worker {
		return newWorkerMock("worker_0", func(ctx context.Context, started chan<- struct{}) error {
			close(started)
			return nil
		})
	}

	ctx, cancel := context.WithCancel(s.ctx)
	cancel()

	errorCh := s.runInBackground(ctx, newWorker(), newWorker(), newWorker())
	err := s.waitWithTimeout(s.ctx, errorCh)
	s.Require().ErrorIs(err, context.Canceled)
}

func (s *ServiceTestSuite) Test_Run_Without_Started_Notified() {
	var startedWorkersCnt atomic.Int32
	cancelledWorkers := make([]string, 0, 2)

	newNotifyingWorker := func(name string) Worker {
		return newWorkerMock(name, func(ctx context.Context, started chan<- struct{}) error {
			startedWorkersCnt.Add(1)
			close(started)
			<-ctx.Done()
			cancelledWorkers = append(cancelledWorkers, name)
			return nil
		})
	}

	workers := []Worker{
		newNotifyingWorker("worker_0"),
		newWorkerMock("worker_1", func(ctx context.Context, started chan<- struct{}) error {
			startedWorkersCnt.Add(1)
			// do not close started ch, just terminate quickly
			return nil
		}),
		newNotifyingWorker("worker_2"),
	}

	ctx, cancel := context.WithCancel(s.ctx)
	errorCh := s.runInBackground(ctx, workers...)

	s.Require().Eventually(func() bool {
		return startedWorkersCnt.Load() == 3
	}, 3*time.Second, 10*time.Millisecond)

	cancel()
	err := s.waitWithTimeout(s.ctx, errorCh)
	s.Require().ErrorIs(err, context.Canceled)

	s.Require().Equal(
		[]string{"worker_2", "worker_0"}, cancelledWorkers, "[worker_2, worker_0] should be cancelled",
	)
}

func (s *ServiceTestSuite) Test_Cancel_Worker_With_Long_Startup() {
	newWorker := func() Worker {
		return newWorkerMock("worker_0", func(ctx context.Context, started chan<- struct{}) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			// Emulating long-running startup
			case <-time.After(time.Minute):
			}

			close(started)
			return nil
		})
	}

	ctx, cancel := context.WithCancel(s.ctx)
	errorCh := s.runInBackground(ctx, newWorker())
	cancel()

	err := s.waitWithTimeout(s.ctx, errorCh)
	s.Require().ErrorIs(err, context.Canceled)
}

const idxNoError = -1

type runAndCancelTestCase struct {
	name            string
	workerCount     int32
	faultyWorkerIdx int32
	workerPanics    bool
}

func (c *runAndCancelTestCase) noExpectedError() bool {
	return c.faultyWorkerIdx == idxNoError
}

// Test_Run_And_Cancel_In_Order verifies that workers are started in sequentially in forward order
// and terminated in reverse order upon context cancellation.
func (s *ServiceTestSuite) Test_Run_And_Cancel_In_Order() {
	testCases := []runAndCancelTestCase{
		{"Single_Worker_No_Error", 1, idxNoError, false},
		{"Multiple_Workers_No_Error", 10, idxNoError, false},

		{"Single_Worker_Returns_Error", 1, 0, false},
		{"Single_Worker_Panics", 1, 0, true},

		{"Multiple_Workers_First_Returns_Error", 10, 0, false},
		{"Multiple_Workers_First_Panics", 10, 0, true},

		{"Multiple_Workers_Middle_Returns_Error", 10, 3, false},
		{"Multiple_Workers_Middle_Panics", 10, 7, true},

		{"Multiple_Workers_Last_Returns_Error", 10, 9, false},
		{"Multiple_Workers_Last_Panics", 10, 9, true},
	}

	for _, testCase := range testCases {
		check.PanicIff(testCase.workerCount <= 0, "workerCount must positive")
		check.PanicIfNotf(
			testCase.faultyWorkerIdx >= 0 && testCase.faultyWorkerIdx <= testCase.workerCount ||
				testCase.faultyWorkerIdx == idxNoError,
			"faultyWorkerIdx must be in range [0, workerCount) or equal to idxNoError",
		)

		s.Run(testCase.name, func() {
			s.runWorkersAndReturnErrAt(testCase)
		})
	}
}

// runWorkersAndReturnErrAt runs a specified number of workers and returns the error from a designated faulty worker.
func (s *ServiceTestSuite) runWorkersAndReturnErrAt(testCase runAndCancelTestCase) {
	s.T().Helper()
	startupErr := errors.New("worker failed before started")

	activeWorkersStack := make([]string, 0, testCase.workerCount)
	var activeWorkersCnt atomic.Int32

	workers := make([]Worker, 0, testCase.workerCount)
	for i := range testCase.workerCount {
		var worker Worker
		worker = newWorkerMock(
			fmt.Sprintf("worker_%d", i),
			func(ctx context.Context, started chan<- struct{}) error {
				if i == testCase.faultyWorkerIdx {
					if testCase.workerPanics {
						panic("worker panic")
					}
					return startupErr
				}

				activeWorkersCnt.Add(1)
				activeWorkersStack = append(activeWorkersStack, worker.Name())
				close(started)

				<-ctx.Done()
				s.Equal(
					worker.Name(),
					activeWorkersStack[len(activeWorkersStack)-1],
					"worker termination order is not preserved")
				activeWorkersStack = activeWorkersStack[:len(activeWorkersStack)-1]
				return ctx.Err()
			},
		)
		workers = append(workers, worker)
	}

	ctx, cancel := context.WithCancel(s.ctx)
	defer cancel()

	errorCh := s.runInBackground(ctx, workers...)

	var expectedActiveCount int32
	if testCase.noExpectedError() {
		expectedActiveCount = testCase.workerCount
	} else {
		expectedActiveCount = testCase.faultyWorkerIdx
	}

	s.Require().Eventually(func() bool {
		return activeWorkersCnt.Load() == expectedActiveCount
	}, 3*time.Second, 10*time.Millisecond)

	if testCase.noExpectedError() {
		cancel()
	}

	err := s.waitWithTimeout(s.ctx, errorCh)
	switch {
	case testCase.noExpectedError():
		s.Require().ErrorIs(err, context.Canceled)
	case testCase.workerPanics:
		s.Require().ErrorContains(err, "worker panic")
	default:
		s.Require().ErrorIs(err, startupErr)
	}

	s.Require().Empty(activeWorkersStack)
}

func (s *ServiceTestSuite) Test_Run_Worker_Error_After_Started() {
	testCases := []struct {
		name                 string
		waitForOthersToStart bool
		workerPanics         bool
	}{
		{"Return_Error_Before_Others_Started", false, false},
		{"Return_Error_After_Others_Started", true, false},
		{"Panic_Before_Others_Started", false, true},
		{"Panic_After_Others_Started", true, true},
	}

	for _, testCase := range testCases {
		s.Run(testCase.name, func() {
			s.runWorkerWithErrAfterStarted(testCase.waitForOthersToStart, testCase.workerPanics)
		})
	}
}

func (s *ServiceTestSuite) runWorkerWithErrAfterStarted(waitForOthersToStart bool, workerPanics bool) {
	s.T().Helper()
	cancelledWorkers := make([]string, 0, 2)
	workerErr := errors.New("worker error")

	var startedGroup sync.WaitGroup
	startedGroup.Add(2)

	nonFailingWorker := func(name string) Worker {
		return newWorkerMock(name, func(ctx context.Context, started chan<- struct{}) error {
			close(started)
			startedGroup.Done()
			<-ctx.Done()
			cancelledWorkers = append(cancelledWorkers, name)
			return nil
		})
	}

	workers := []Worker{
		nonFailingWorker("worker_0"),
		newWorkerMock("worker_1", func(ctx context.Context, started chan<- struct{}) error {
			close(started)
			if waitForOthersToStart {
				startedGroup.Wait()
			}
			if workerPanics {
				panic("worker panic")
			}
			return workerErr
		}),
		nonFailingWorker("worker_2"),
	}

	errorCh := s.runInBackground(s.ctx, workers...)
	err := s.waitWithTimeout(s.ctx, errorCh)

	if workerPanics {
		s.Require().ErrorContains(err, "worker panic")
	} else {
		s.Require().ErrorIs(err, workerErr)
	}

	s.Require().Equal(
		[]string{"worker_2", "worker_0"}, cancelledWorkers, "[worker_2, worker_0] should be cancelled",
	)
}

func (s *ServiceTestSuite) Test_Stop_Service() {
	var startedWorkersCnt atomic.Int32
	var stoppedWorkersCnt atomic.Int32

	newWorker := func(name string) Worker {
		return newWorkerMock(name, func(ctx context.Context, started chan<- struct{}) error {
			close(started)
			startedWorkersCnt.Add(1)
			<-ctx.Done()
			stoppedWorkersCnt.Add(1)
			return ctx.Err()
		})
	}

	service := NewService(s.logger, newWorker("worker_0"), newWorker("worker_1"), newWorker("worker_2"))
	errorCh := make(chan error, 1)
	go func() {
		errorCh <- service.Run(s.ctx)
		close(errorCh)
	}()

	s.Require().Eventually(func() bool {
		return startedWorkersCnt.Load() == 3
	}, 3*time.Second, 10*time.Millisecond)

	select {
	case <-time.After(serviceTerminateTimeout):
		s.Fail("service did not stop in time")
	case <-service.Stop():
	}

	err := s.waitWithTimeout(s.ctx, errorCh)
	s.Require().ErrorIs(err, context.Canceled)
	s.Require().Equal(int32(3), stoppedWorkersCnt.Load())
}

func (s *ServiceTestSuite) runInBackground(ctx context.Context, workers ...Worker) <-chan error {
	service := NewService(s.logger, workers...)
	errorCh := make(chan error, 1)
	go func() {
		errorCh <- service.Run(ctx)
		close(errorCh)
	}()
	return errorCh
}

func (s *ServiceTestSuite) waitWithTimeout(ctx context.Context, errorCh <-chan error) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errorCh:
		return err
	case <-time.After(serviceTerminateTimeout):
		err := errors.New("service did not terminate in time")
		s.Fail(err.Error())
		return err
	}
}

func newWorkerMock(
	name string,
	runFunc func(ctx context.Context, started chan<- struct{}) error,
) *WorkerMock {
	return &WorkerMock{
		NameFunc: func() string {
			return name
		},
		RunFunc: runFunc,
	}
}
