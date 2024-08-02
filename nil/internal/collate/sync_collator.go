package collate

import (
	"context"

	"github.com/NilFoundation/nil/common/check"
	"github.com/NilFoundation/nil/common/logging"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/msgpool"
	"github.com/rs/zerolog"
)

type syncCollator struct {
	msgPool MsgPool
	logger  zerolog.Logger
}

func NewSyncCollator(msgPool MsgPool, shard types.ShardId) *syncCollator {
	return &syncCollator{
		msgPool: msgPool,
		logger: logging.NewLogger("collator").With().
			Stringer(logging.FieldShardId, shard).
			Logger(),
	}
}

func (s *syncCollator) Run(ctx context.Context) error {
	s.logger.Info().Msg("Starting sync")
	return nil
}

func (s *syncCollator) GetMsgPool() msgpool.Pool {
	pool, ok := s.msgPool.(msgpool.Pool)
	check.PanicIfNotf(ok, "scheduler pool is not a msgpool.Pool")
	return pool
}
