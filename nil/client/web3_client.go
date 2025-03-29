package client

import (
	"context"
)

type Web3Client interface {
	// ClientVersion retrieves the current version of the client.
	ClientVersion(ctx context.Context) (string, error)
}
