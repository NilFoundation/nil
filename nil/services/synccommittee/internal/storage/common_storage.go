package storage

import (
	"fmt"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
)

type commonStorage struct {
	database    db.DB
	retryRunner common.RetryRunner
	logger      logging.Logger
}

func makeCommonStorage(
	database db.DB,
	logger logging.Logger,
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
