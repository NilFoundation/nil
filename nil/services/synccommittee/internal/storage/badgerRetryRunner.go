package storage

import (
	"errors"
	"time"

	"github.com/NilFoundation/badger/v4"
	"github.com/NilFoundation/nil/nil/common"
	"github.com/rs/zerolog"
)

func badgerRetryRunner(logger zerolog.Logger) common.RetryRunner {
	return common.NewRetryRunner(
		common.RetryConfig{
			ShouldRetry: func(_ uint32, err error) bool {
				return errors.Is(err, badger.ErrConflict)
			},
			NextDelay: func(_ uint32) time.Duration {
				delay, err := common.RandomDelay(20*time.Millisecond, 100*time.Millisecond)
				if err != nil {
					logger.Error().Err(err).Msg("failed to generate task storage retry delay")
					return 100 * time.Millisecond
				}
				return *delay
			},
		},
		logger,
	)
}
