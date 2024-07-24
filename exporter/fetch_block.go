package exporter

import (
	"errors"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/logging"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/rpc/transport"
)

var logger = logging.NewLogger("fetch-block")

var ErrBlockNotFound = errors.New("block not found")

func (cfg *Cfg) FetchBlockByNumber(shardId types.ShardId, blockId transport.BlockNumber) (*types.BlockWithExtractedData, error) {
	block, err := cfg.Client.GetDebugBlock(shardId, blockId, true)
	if err != nil {
		return nil, err
	}
	if block == nil {
		return nil, ErrBlockNotFound
	}
	return block.DecodeHexAndSSZ()
}

func (cfg *Cfg) FetchBlockByHash(shardId types.ShardId, blockHash common.Hash) (*types.BlockWithExtractedData, error) {
	block, err := cfg.Client.GetDebugBlock(shardId, blockHash, true)
	if err != nil {
		return nil, err
	}
	if block == nil {
		return nil, ErrBlockNotFound
	}
	return block.DecodeHexAndSSZ()
}

func (cfg *Cfg) FetchLastBlock(shardId types.ShardId) (*types.BlockWithExtractedData, error) {
	latestBlock, err := cfg.FetchBlockByNumber(shardId, transport.LatestBlockNumber)
	if err != nil {
		return nil, err
	}
	return latestBlock, nil
}

func (cfg *Cfg) FetchShards() ([]types.ShardId, error) {
	return cfg.Client.GetShardIdList()
}
