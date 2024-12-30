package common

import (
	"context"
	"crypto/rand"
	"fmt"
	"log"
	"math/big"
	"time"

	"github.com/rs/zerolog"
)

type (
	RetryPolicyFunc func(attempt uint32, err error) bool
	NextDelayFunc   func(attempt uint32) time.Duration
)

type RetryConfig struct {
	ShouldRetry RetryPolicyFunc
	NextDelay   NextDelayFunc
}

type RetryRunner struct {
	config RetryConfig
	logger zerolog.Logger
}

func NewRetryRunner(config RetryConfig, logger zerolog.Logger) RetryRunner {
	return RetryRunner{
		config: config,
		logger: logger,
	}
}

func (r *RetryRunner) Do(ctx context.Context, action func(ctx context.Context) error) error {
	attemptNumber := uint32(0)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			attemptNumber++
			err := action(ctx)

			if err == nil || !r.config.ShouldRetry(attemptNumber, err) {
				return err
			}

			delay := r.config.NextDelay(attemptNumber)
			r.logger.Warn().Err(err).Msgf("operation failed, retrying in %s (try %d)", delay, attemptNumber)

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
				break
			}
		}
	}
}

func LimitRetries(maxRetries uint32) RetryPolicyFunc {
	return func(attemptNumber uint32, _ error) bool {
		return attemptNumber < maxRetries
	}
}

func ComposeRetryPolicies(policies ...RetryPolicyFunc) RetryPolicyFunc {
	return func(attempt uint32, err error) bool {
		for _, policy := range policies {
			if !policy(attempt, err) {
				return false
			}
		}
		return true
	}
}

func ExponentialDelay(baseDelay, maxDelay time.Duration) NextDelayFunc {
	if baseDelay > maxDelay {
		log.Panicf("baseDelay %s > maxDelay %s", baseDelay, maxDelay)
	}

	return func(attemptNumber uint32) time.Duration {
		result := time.Duration(1)
		for range attemptNumber {
			result *= baseDelay
			if result >= maxDelay {
				result = maxDelay
				break
			}
		}
		return result
	}
}

func RandomDelay(minDelay, maxDelay time.Duration) (*time.Duration, error) {
	if minDelay > maxDelay {
		return nil, fmt.Errorf("minDelay %s > maxDelay %s", minDelay, maxDelay)
	}

	maxDelta := big.NewInt(int64(maxDelay - minDelay + 1))
	randomDelta, err := rand.Int(rand.Reader, maxDelta)
	if err != nil {
		return nil, fmt.Errorf("failed to generate random delay: %w", err)
	}

	delay := minDelay + time.Duration(randomDelta.Int64())
	return &delay, nil
}
