package srv

import "context"

//go:generate bash ../scripts/generate_mock.sh Worker

type Worker interface {
	// Name returns the name of the Worker. This is typically used for logging and identification.
	Name() string

	// Run starts the worker, signaling its initialization through the started channel.
	Run(ctx context.Context, started chan<- struct{}) error
}
