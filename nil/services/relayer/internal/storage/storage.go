package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/jonboulle/clockwork"
)

type BaseStorage struct {
	Database    db.DB
	RetryRunner common.RetryRunner
	Clock       clockwork.Clock
	Logger      logging.Logger
	Metrics     TableMetrics
}

func NewBaseStorage(
	ctx context.Context,
	database db.DB,
	clock clockwork.Clock,
	logger logging.Logger,
	metrics TableMetrics,
) *BaseStorage {
	return &BaseStorage{
		Database: database,
		RetryRunner: common.NewRetryRunner(
			common.RetryConfig{
				ShouldRetry: common.ComposeRetryPolicies(
					common.LimitRetries(10),
					common.DoNotRetryIf(ErrKeyExists, ErrSerializationFailed),
				),
				NextDelay: common.DelayJitter(20*time.Millisecond, 100*time.Millisecond, logger),
			},
			logger,
		),
		Clock:   clock,
		Logger:  logger,
		Metrics: metrics,
	}
}

func (*BaseStorage) Commit(tx db.RwTx, onCommitted func()) error {
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	if onCommitted != nil {
		onCommitted()
	}
	return nil
}
