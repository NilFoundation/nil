package common

import (
	"context"
	"time"
)

// WaitFor repeatedly calls the given function until it returns true or an error.
func WaitFor(ctx context.Context, timeout, tick time.Duration, f func(ctx context.Context) bool) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(tick)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if f(ctx) {
				return nil
			}
		}
	}
}

// WaitForValue repeatedly calls the given function until it returns true or an error.
// In case function return some data not equal to nil, it returns true.
func WaitForValue[T any](
	ctx context.Context,
	timeout,
	tick time.Duration,
	f func(ctx context.Context) (*T, error),
) (*T, error) {
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
