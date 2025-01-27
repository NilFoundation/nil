package srv

import "context"

//go:generate bash ../scripts/generate_mock.sh Worker

type Worker interface {
	Name() string
	Run(ctx context.Context) error
}
