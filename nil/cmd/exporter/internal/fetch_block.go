package internal

import (
	"context"
	"errors"

	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/types"
)

var logger = logging.NewLogger("fetch-block")

var ErrBlockNotFound = errors.New("block not found")

func (cfg *Cfg) FetchBlocks(ctx context.Context, shardId types.ShardId, fromId types.BlockNumber, toId types.BlockNumber) ([]*types.BlockWithExtractedData, error) {
	rawBlocks, err := cfg.Client.GetDebugBlocksRange(ctx, shardId, fromId, toId, true, int(toId-fromId))
	if err != nil {
		return nil, err
	}

	result := make([]*types.BlockWithExtractedData, len(rawBlocks))
	for i, raw := range rawBlocks {
		check.PanicIfNot(raw != nil)
		result[i], err = raw.DecodeSSZ()
		if err != nil {
			return nil, err
		}
	}

	return result, nil
}

func (cfg *Cfg) FetchBlock(ctx context.Context, shardId types.ShardId, blockId any) (*types.BlockWithExtractedData, error) {
	latestBlock, err := cfg.Client.GetDebugBlock(ctx, shardId, blockId, true)
	if err != nil {
		return nil, err
	}
	if latestBlock == nil {
		return nil, ErrBlockNotFound
	}
	return latestBlock.DecodeSSZ()
}

func (cfg *Cfg) FetchShards(ctx context.Context) ([]types.ShardId, error) {
	return cfg.Client.GetShardIdList(ctx)
}
