package internal

import (
	"errors"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/transport"
)

var logger = logging.NewLogger("fetch-block")

var ErrBlockNotFound = errors.New("block not found")

func (cfg *Cfg) FetchBlock(shardId types.ShardId, blockId any) (*types.BlockWithExtractedData, error) {
	block, err := cfg.Client.GetDebugBlock(shardId, blockId, true)
	if err != nil {
		return nil, err
	}
	if block == nil {
		return nil, ErrBlockNotFound
	}
	return block.DecodeHexAndSSZ()
}

func (cfg *Cfg) FetchLastBlock(shardId types.ShardId) (*types.BlockWithExtractedData, error) {
	latestBlock, err := cfg.FetchBlock(shardId, transport.LatestBlockNumber)
	if err != nil {
		return nil, err
	}
	return latestBlock, nil
}

func (cfg *Cfg) FetchShards() ([]types.ShardId, error) {
	return cfg.Client.GetShardIdList()
}
