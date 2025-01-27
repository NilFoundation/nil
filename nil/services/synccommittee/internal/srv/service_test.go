package srv

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/suite"
)

type ServiceTestSuite struct {
	suite.Suite

	ctx          context.Context
	cancellation context.CancelFunc

	logger zerolog.Logger
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

func (s *ServiceTestSuite) Test_Run() {
	workerErr := errors.New("worker error")

	testCases := []runTestCase{
		{
			name: "Single_Worker_Success",
			workers: []Worker{
				newWorkerMock(func(ctx context.Context) error {
					return nil
				}),
			},
			expectedErr: nil,
		},
		{
			name: "Single_Worker_Error",
			workers: []Worker{
				newWorkerMock(func(ctx context.Context) error {
					return workerErr
				}),
			},
			expectedErr: workerErr,
		},
		{
			name: "Multiple_Workers_Mixed_Results",
			workers: []Worker{
				newWorkerMock(func(ctx context.Context) error {
					return nil
				}),
				newWorkerMock(func(ctx context.Context) error {
					return workerErr
				}),
			},
			expectedErr: workerErr,
		},
	}

	for _, testCase := range testCases {
		s.Run(testCase.name, func() {
			s.runService(testCase)
		})
	}
}

func (s *ServiceTestSuite) runService(testCase runTestCase) {
	s.T().Helper()
	service := NewService(DefaultConfig(), s.logger, testCase.workers...)

	err := service.Run(s.ctx)
	s.Require().ErrorIs(err, testCase.expectedErr)
}

func (s *ServiceTestSuite) Test_Run_Cancellation() {
	worker := newWorkerMock(func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	})

	service := NewService(DefaultConfig(), s.logger, worker)

	var wg sync.WaitGroup
	wg.Add(1)

	ctx, cancel := context.WithCancel(s.ctx)

	go func() {
		defer wg.Done()
		err := service.Run(ctx)
		s.ErrorIs(err, context.Canceled)
	}()

	cancel()
	wg.Wait()
}

func newWorkerMock(runFunc func(ctx context.Context) error) *WorkerMock {
	return &WorkerMock{
		NameFunc: func() string {
			return "test_worker"
		},
		RunFunc: runFunc,
	}
}
