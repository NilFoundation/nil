//go:build test

package testaide

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"
)

func WaitForEndpoint(ctx context.Context, endpoint string) error {
	endpoint = strings.TrimPrefix(endpoint, "tcp://")
	const timeout = 5 * time.Second
	const tick = 100 * time.Millisecond

	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	var lastError error
	for {
		select {
		case <-ctx.Done():
			if lastError != nil {
				return fmt.Errorf("waiting for %s timed out or canceled: %w (last error: %w)", endpoint, ctx.Err(), lastError)
			}
			return fmt.Errorf("waiting for %s timed out or canceled: %w", endpoint, ctx.Err())
		default:
			conn, err := net.Dial("tcp", endpoint)
			if err == nil {
				_ = conn.Close()
				return nil
			}
			lastError = err
			time.Sleep(tick)
		}
	}
}
