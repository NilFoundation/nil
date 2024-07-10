package exporter

import (
	"errors"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/hexutil"
	"github.com/NilFoundation/nil/common/logging"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/rpc/transport"
)

var logger = logging.NewLogger("fetch-block")

func toBlockMsg(shardId types.ShardId, raw map[string]any) (*BlockMsg, error) {
	if raw == nil {
		return nil, errors.New("block not found")
	}

	logger.Debug().Msgf("result map %v", raw)

	hexBody, ok := raw["content"].(string)
	if !ok {
		return nil, errors.New("block content not found")
	}

	hexBytes := hexutil.FromHex(hexBody)

	var block types.Block

	if err := block.UnmarshalSSZ(hexBytes); err != nil {
		return nil, err
	}

	hexInMessagesRaw, ok := raw["inMessages"]
	if !ok {
		return nil, errors.New("block inMessages not found")
	}
	hexInMessages, ok := hexInMessagesRaw.([]any)
	if !ok {
		return nil, errors.New("cannot convert inMessages to []any")
	}
	inMessages := make([]*types.Message, 0)
	for _, hexMessage := range hexInMessages {
		message := types.Message{}
		stringMsg, ok := hexMessage.(string)
		if !ok {
			return nil, errors.New("cannot convert message to string")
		}
		hexMessageBytes := hexutil.FromHex(stringMsg)
		if err := message.UnmarshalSSZ(hexMessageBytes); err != nil {
			return nil, err
		}
		inMessages = append(inMessages, &message)
	}

	hexOutMessagesRaw, ok := raw["outMessages"]
	if !ok {
		return nil, errors.New("block outMessages not found")
	}
	hexOutMessages, ok := hexOutMessagesRaw.([]any)
	if !ok {
		return nil, errors.New("cannot convert outMessages to []any")
	}
	outMessages := make([]*types.Message, 0)
	for _, hexMessage := range hexOutMessages {
		message := types.Message{}
		strOutMsg, ok := hexMessage.(string)
		if !ok {
			return nil, errors.New("cannot convert message to strOutg")
		}
		hexMessageBytes := hexutil.FromHex(strOutMsg)
		if err := message.UnmarshalSSZ(hexMessageBytes); err != nil {
			return nil, err
		}
		outMessages = append(outMessages, &message)
	}

	hexReceiptsRaw, ok := raw["receipts"]
	if !ok {
		return nil, errors.New("block receipts not found")
	}
	hexReceipts, ok := hexReceiptsRaw.([]any)
	if !ok {
		return nil, errors.New("cannot convert receipts to []any")
	}
	receipts := make([]*types.Receipt, 0)
	for _, hexReceipt := range hexReceipts {
		receipt := types.Receipt{}
		stringMsg, ok := hexReceipt.(string)
		if !ok {
			return nil, errors.New("cannot convert receipt to string")
		}
		hexReceiptBytes := hexutil.FromHex(stringMsg)
		if err := receipt.UnmarshalSSZ(hexReceiptBytes); err != nil {
			return nil, err
		}
		receipts = append(receipts, &receipt)
	}

	positionsRaw, ok := raw["positions"]
	if !ok {
		return nil, errors.New("block positions not found")
	}

	positions, ok := positionsRaw.([]any)
	if !ok {
		return nil, errors.New("cannot convert positions to []uint64")
	}

	positionsUint64 := make([]uint64, 0)

	for _, position := range positions {
		u, ok := position.(float64)
		if !ok {
			return nil, errors.New("cannot convert position to uint64")
		}
		positionsUint64 = append(positionsUint64, uint64(u))
	}

	result := &BlockMsg{
		Block:       &block,
		InMessages:  inMessages,
		OutMessages: outMessages,
		Receipts:    receipts,
		Shard:       shardId,
		Positions:   positionsUint64,
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
