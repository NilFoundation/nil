package concurrent

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRunWithTimeout_AllFunctionsSucceed(t *testing.T) {
	t.Parallel()

	err := RunWithTimeout(
		t.Context(),
		1*time.Second,
		WithSource(func(ctx context.Context) error {
			return nil
		}),
		WithSource(func(ctx context.Context) error {
			return nil
		}))
	require.NoError(t, err)
}

var errTest = errors.New("test error")

func TestRunWithTimeout_FunctionFails(t *testing.T) {
	t.Parallel()
	synctestRun(func() {
		err := RunWithTimeout(
			t.Context(),
			1*time.Second,
			WithSource(func(ctx context.Context) error {
				return errTest
			}),
			WithSource(func(ctx context.Context) error {
				time.Sleep(50 * time.Millisecond)
				synctestWait()
				return ctx.Err()
			}))
		var exectutionError *ExecutionError
		require.ErrorAs(t, err, &exectutionError)
		require.ErrorIs(t, exectutionError.Err, errTest)
		errorLines := strings.Split(exectutionError.Error(), "\n")
		require.Contains(t, errorLines[0], errTest.Error())
		require.Contains(t, errorLines[2], "nil/common/concurrent/utils_test.go:36")
	})
}

func TestRunWithTimeout_TimeoutExceededWithContextCheck(t *testing.T) {
	t.Parallel()
	synctestRun(func() {
		err := RunWithTimeout(
			t.Context(),
			10*time.Millisecond,
			WithSource(func(ctx context.Context) error {
				time.Sleep(50 * time.Millisecond)
				synctestWait()
				return ctx.Err()
			}))
		require.Error(t, err)
		require.Contains(t, err.Error(), "context deadline exceeded")
	})
}

func TestRunWithTimeout_TimeoutExceededWithoutContextCheck(t *testing.T) {
	t.Parallel()

	synctestRun(func() {
		err := RunWithTimeout(
			t.Context(),
			10*time.Millisecond,
			WithSource(func(ctx context.Context) error {
				time.Sleep(50 * time.Millisecond)
				synctestWait()
				return nil
			}))
		require.NoError(t, err)
	})
}
