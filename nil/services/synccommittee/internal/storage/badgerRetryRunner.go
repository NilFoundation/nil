package storage

import (
	"time"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/rs/zerolog"
)

const (
	badgerDefaultRetryLimit = 20
)

func badgerRetryRunner(
	logger zerolog.Logger,
	additionalPolicies ...common.RetryPolicyFunc,
) common.RetryRunner {
	retryPolicy := common.ComposeRetryPolicies(
		append(
			[]common.RetryPolicyFunc{
				common.DoNotRetryIf(ErrSerializationFailed),
				common.LimitRetries(badgerDefaultRetryLimit),
			},
			additionalPolicies...,
		)...,
	)

	return common.NewRetryRunner(
		common.RetryConfig{
			ShouldRetry: retryPolicy,
			NextDelay:   common.DelayJitter(20*time.Millisecond, 100*time.Millisecond, logger),
		},
		logger,
	)
}
