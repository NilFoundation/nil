package storage

import (
	"fmt"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/rs/zerolog"
)

type commonStorage struct {
	database    db.DB
	retryRunner common.RetryRunner
	logger      zerolog.Logger
}

func makeCommonStorage(
	database db.DB,
	logger zerolog.Logger,
	additionalRetryPolicies ...common.RetryPolicyFunc,
) commonStorage {
	return commonStorage{
		database:    database,
		retryRunner: badgerRetryRunner(logger, additionalRetryPolicies...),
		logger:      logger,
	}
}

func (*commonStorage) commit(tx db.RwTx) error {
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
}
