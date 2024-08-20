package concurrent

import (
	"context"
	"sync"
	"time"

	"github.com/NilFoundation/nil/nil/common/check"
)

type Func = func(context.Context) error

// RunWithTimeout calls each given function in a separate goroutine and waits for them to finish.
// It logs a fatal message if an error occurred.
// If timeout is positive, it is added to the context. Otherwise, it is ignored.
// Note that RunWithTimeout does not forcefully terminate the goroutines;
// your functions should be able to handle context cancellation.
func RunWithTimeout(ctx context.Context, timeout time.Duration, fs ...Func) error {
	var wg sync.WaitGroup

	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	for _, f := range fs {
		wg.Add(1)

		go func(fn Func) {
			defer wg.Done()

			err := fn(ctx)
			// todo: decide on what to do with other goroutines
			check.PanicIfErr(err)
		}(f) // to avoid loop-variable reuse in goroutines
	}

	wg.Wait()
	return nil
}

// WaitFor repeatedly calls the given function until it returns true or an error.
// In case function return some data not equal to nil, it returns true.
func WaitFor[T any](ctx context.Context, timeout, tick time.Duration, f func(ctx context.Context) (*T, error)) (*T, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(tick)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			res, err := f(ctx)
			if err != nil {
				return res, err
			}
			if res != nil {
				return res, nil
			}
		}
	}
}

// Run calls RunWithTimeout without a timeout.
func Run(ctx context.Context, fs ...Func) error {
	return RunWithTimeout(ctx, 0, fs...)
}

// RunTickerLoop runs a loop that executes a function at regular intervals
func RunTickerLoop(ctx context.Context, interval time.Duration, onTick func(context.Context)) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			onTick(ctx)
		case <-ctx.Done():
			return
		}
	}
}
