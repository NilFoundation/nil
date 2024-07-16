package exporter

import (
	"errors"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/hexutil"
	"github.com/NilFoundation/nil/common/logging"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/rpc/jsonrpc"
	"github.com/NilFoundation/nil/rpc/transport"
)

var logger = logging.NewLogger("fetch-block")

func toBlockMsg(shardId types.ShardId, rawBlock *jsonrpc.RPCRawBlock) (*BlockMsg, error) {
	if rawBlock == nil {
		return nil, errors.New("block not found")
	}

	logger.Debug().Msgf("result map %v", rawBlock)

	hexBytes := hexutil.FromHex(rawBlock.Content)

	var block types.Block
	if err := block.UnmarshalSSZ(hexBytes); err != nil {
		return nil, err
	}

	inMessages, err := rawBlock.InMessages()
	if err != nil {
		return nil, err
	}

	outMessages, err := rawBlock.OutMessages()
	if err != nil {
		return nil, err
	}

	receipts, err := rawBlock.Receipts()
	if err != nil {
		return nil, err
	}

	result := &BlockMsg{
		Block:       &block,
		InMessages:  inMessages,
		OutMessages: outMessages,
		Receipts:    receipts,
		Shard:       shardId,
		Positions:   rawBlock.Positions,
	}

	logger.Debug().
		Stringer(logging.FieldBlockHash, result.Block.Hash()).
		Msg("Fetched block")

	return result, nil
}

func (cfg *Cfg) FetchBlockByNumber(shardId types.ShardId, blockId transport.BlockNumber) (*BlockMsg, error) {
	raw, err := cfg.Client.GetRawBlock(shardId, blockId, true)
	if err != nil {
		return nil, err
	}
	return toBlockMsg(shardId, raw)
}

func (cfg *Cfg) FetchBlockByHash(shardId types.ShardId, blockHash common.Hash) (*BlockMsg, error) {
	raw, err := cfg.Client.GetRawBlock(shardId, blockHash, true)
	if err != nil {
		return nil, err
	}
	return toBlockMsg(shardId, raw)
}

func (cfg *Cfg) FetchLastBlock(shardId types.ShardId) (*BlockMsg, error) {
	latestBlock, err := cfg.FetchBlockByNumber(shardId, transport.LatestBlockNumber)
	if err != nil {
		return nil, err
	}
	return latestBlock, nil
}

func (cfg *Cfg) FetchShards() ([]types.ShardId, error) {
	return cfg.Client.GetShardIdList()
}
