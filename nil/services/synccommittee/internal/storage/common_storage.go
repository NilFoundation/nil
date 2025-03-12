package storage

import (
	"fmt"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/rs/zerolog"
)

type CommonStorage struct {
	Database    db.DB
	RetryRunner common.RetryRunner
	Logger      zerolog.Logger
}

func NewCommonStorage(
	database db.DB,
	logger zerolog.Logger,
	additionalRetryPolicies ...common.RetryPolicyFunc,
) CommonStorage {
	return CommonStorage{
		Database:    database,
		RetryRunner: badgerRetryRunner(logger, additionalRetryPolicies...),
		Logger:      logger,
	}
}

func (*CommonStorage) Commit(tx db.RwTx) error {
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
}
