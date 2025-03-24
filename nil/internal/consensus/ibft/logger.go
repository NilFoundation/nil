package ibft

import (
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/types"
)

type ibftLogger struct {
	shardId types.ShardId
	logger  logging.Logger
}

func (l *ibftLogger) Info(msg string, args ...any) {
	if l.shardId != types.BaseShardId {
		return
	}
	l.logger.Info().Fields(args).Msg(msg)
}

func (l *ibftLogger) Debug(msg string, args ...any) {
	if l.shardId != types.BaseShardId {
		return
	}
	l.logger.Debug().Fields(args).Msg(msg)
}

func (l *ibftLogger) Error(msg string, args ...any) {
	if l.shardId != types.BaseShardId {
		return
	}
	l.logger.Error().Fields(args).Msg(msg)
}
