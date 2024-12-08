//go:build test

package testaide

import (
	"context"
	"net"
	"strings"
	"time"
)

func WaitForEndpoint(ctx context.Context, endpoint string) error {
	endpoint = strings.TrimPrefix(endpoint, "tcp://")

	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			conn, err := net.Dial("tcp", endpoint)
			if err == nil {
				_ = conn.Close()
				return nil
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
}
