package concurrent

import (
	"context"
	"sync"
	"time"

	"github.com/NilFoundation/nil/common/check"
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

// Run calls RunWithTimeout without a timeout.
func Run(ctx context.Context, fs ...Func) error {
	return RunWithTimeout(ctx, 0, fs...)
}
