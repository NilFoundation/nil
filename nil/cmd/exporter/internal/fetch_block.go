package internal

import (
	"errors"

	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
	"github.com/NilFoundation/nil/nil/services/rpc/transport"
)

var logger = logging.NewLogger("fetch-block")

var ErrBlockNotFound = errors.New("block not found")

func (cfg *Cfg) FetchBlocks(shardId types.ShardId, fromId types.BlockNumber, toId types.BlockNumber) ([]*types.BlockWithExtractedData, error) {
	batch := cfg.Client.CreateBatchRequest()

	for i := fromId; i < toId; i++ {
		if _, err := batch.GetDebugBlock(shardId, transport.BlockNumber(i), true); err != nil {
			return nil, err
		}
	}

	resp, err := cfg.Client.BatchCall(batch)
	if err != nil {
		return nil, err
	}

	result := make([]*types.BlockWithExtractedData, len(resp))
	for i, b := range resp {
		block, ok := b.(*jsonrpc.HexedDebugRPCBlock)
		check.PanicIfNot(ok)
		result[i], err = block.DecodeHexAndSSZ()
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
	return latestBlock.DecodeHexAndSSZ()
}

func (cfg *Cfg) FetchShards() ([]types.ShardId, error) {
	return cfg.Client.GetShardIdList()
}
