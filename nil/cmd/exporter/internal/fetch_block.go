package internal

import (
	"errors"

	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/types"
)

var logger = logging.NewLogger("fetch-block")

var ErrBlockNotFound = errors.New("block not found")

func (cfg *Cfg) FetchBlocks(shardId types.ShardId, fromId types.BlockNumber, toId types.BlockNumber) ([]*types.BlockWithExtractedData, error) {
	rawBlocks, err := cfg.Client.GetDebugBlocksRange(shardId, fromId, toId, true, int(toId-fromId))
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

func (cfg *Cfg) FetchBlock(shardId types.ShardId, blockId any) (*types.BlockWithExtractedData, error) {
	latestBlock, err := cfg.Client.GetDebugBlock(shardId, blockId, true)
	if err != nil {
		return nil, err
	}
	if latestBlock == nil {
		return nil, ErrBlockNotFound
	}
	return latestBlock.DecodeSSZ()
}

func (cfg *Cfg) FetchShards() ([]types.ShardId, error) {
	return cfg.Client.GetShardIdList()
}
