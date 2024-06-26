package exporter

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/hexutil"
	"github.com/NilFoundation/nil/common/logging"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/rpc/transport"
)

var logger = logging.NewLogger("fetch-block")

type request struct {
	Jsonrpc string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  []any  `json:"params"`
	Id      int    `json:"id"`
}

type blockResponse struct {
	Jsonrpc string         `json:"jsonrpc"`
	Result  map[string]any `json:"result"`
	Id      int            `json:"id"`
}

type blockShardIdsResponse struct {
	Jsonrpc string          `json:"jsonrpc"`
	Result  []types.ShardId `json:"result"`
	Id      int             `json:"id"`
}

func (cfg *Cfg) fetchBlockData(ctx context.Context, requestBody request) (*BlockMsg, error) {
	requestBytesBody, err := json.Marshal(requestBody)
	if err != nil {
		return nil, err
	}
	endpoint := cfg.pickAPIEndpoint()
	requestWithCtx, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewBuffer(requestBytesBody))
	if err != nil {
		return nil, err
	}
	requestWithCtx.Header.Set("Content-Type", "application/json")
	res, err := cfg.httpClient.Do(requestWithCtx)
	if err != nil {
		return nil, err
	}

	defer func() { _ = res.Body.Close() }()

	// read from response body
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	var bodyResponse blockResponse

	if err = json.Unmarshal(body, &bodyResponse); err != nil {
		return nil, err
	}

	if bodyResponse.Result == nil {
		return nil, errors.New("block not found")
	}

	logger.Debug().Msgf("result map %v", bodyResponse.Result)

	hexBody, ok := bodyResponse.Result["content"].(string)
	if !ok {
		return nil, errors.New("block content not found")
	}

	hexBytes := hexutil.FromHex(hexBody)

	var block types.Block

	if err = block.UnmarshalSSZ(hexBytes); err != nil {
		return nil, err
	}

	hexInMessagesRaw, ok := bodyResponse.Result["inMessages"]
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
		if err = message.UnmarshalSSZ(hexMessageBytes); err != nil {
			return nil, err
		}
		inMessages = append(inMessages, &message)
	}

	hexOutMessagesRaw, ok := bodyResponse.Result["outMessages"]
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
		if err = message.UnmarshalSSZ(hexMessageBytes); err != nil {
			return nil, err
		}
		outMessages = append(outMessages, &message)
	}

	hexReceiptsRaw, ok := bodyResponse.Result["receipts"]
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
		if err = receipt.UnmarshalSSZ(hexReceiptBytes); err != nil {
			return nil, err
		}
		receipts = append(receipts, &receipt)
	}

	positionsRaw, ok := bodyResponse.Result["positions"]
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

	paramsShardId, ok := requestBody.Params[0].(types.ShardId)
	if !ok {
		return nil, errors.New("cannot convert shardId to types.ShardId")
	}

	result := &BlockMsg{
		Block:       &block,
		InMessages:  inMessages,
		OutMessages: outMessages,
		Receipts:    receipts,
		Shard:       paramsShardId,
		Positions:   positionsUint64,
	}

	logger.Debug().
		Stringer(logging.FieldBlockHash, result.Block.Hash()).
		Msg("Fetched block")

	return result, nil
}

func (cfg *Cfg) FetchBlockByNumber(ctx context.Context, shardId types.ShardId, blockId transport.BlockNumber) (*BlockMsg, error) {
	requestBody := request{Id: 1, Jsonrpc: "2.0", Method: "debug_getBlockByNumber", Params: []any{shardId, blockId, true}}
	return cfg.fetchBlockData(ctx, requestBody)
}

func (cfg *Cfg) FetchBlockByHash(ctx context.Context, shardId types.ShardId, blockHash common.Hash) (*BlockMsg, error) {
	requestBody := request{Id: 1, Jsonrpc: "2.0", Method: "debug_getBlockByHash", Params: []any{shardId, blockHash, true}}
	return cfg.fetchBlockData(ctx, requestBody)
}

func (cfg *Cfg) FetchLastBlock(ctx context.Context, shardId types.ShardId) (*BlockMsg, error) {
	latestBlock, err := cfg.FetchBlockByNumber(ctx, shardId, transport.LatestBlockNumber)
	if err != nil {
		return nil, err
	}
	return latestBlock, nil
}

func (cfg *Cfg) FetchShards(ctx context.Context) ([]types.ShardId, error) {
	requestBody := request{Id: 1, Jsonrpc: "2.0", Method: "eth_getShardIdList", Params: []any{}}
	requestBytesBody, err := json.Marshal(requestBody)
	if err != nil {
		return nil, err
	}
	endpoint := cfg.pickAPIEndpoint()
	requestWithCtx, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewBuffer(requestBytesBody))
	if err != nil {
		return nil, err
	}
	requestWithCtx.Header.Set("Content-Type", "application/json")
	res, err := cfg.httpClient.Do(requestWithCtx)
	if err != nil {
		return nil, err
	}

	defer func() { _ = res.Body.Close() }()

	// read from response body
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	logger.Debug().Msgf("Response body: %s", body)

	var bodyResponse blockShardIdsResponse

	if err = json.Unmarshal(body, &bodyResponse); err != nil {
		return nil, err
	}

	if bodyResponse.Result == nil {
		return nil, errors.New("shards not found")
	}

	return bodyResponse.Result, nil
}
