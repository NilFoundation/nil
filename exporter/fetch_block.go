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
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/rpc/transport"
	"github.com/rs/zerolog/log"
)

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

func (cfg *Cfg) fetchBlockData(ctx context.Context, requestBody request) (*types.Block, error) {
	requestBytesBody, err := json.Marshal(requestBody)
	log.Info().Msgf("Request body: %s", requestBytesBody)
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

	log.Info().Msgf("Response body: %s", body)

	var bodyResponse blockResponse

	if err = json.Unmarshal(body, &bodyResponse); err != nil {
		return nil, err
	}

	if bodyResponse.Result == nil {
		return nil, errors.New("block not found")
	}

	hexBody, ok := bodyResponse.Result["content"].(string)
	common.Require(ok)

	hexBytes := hexutil.FromHex(hexBody)

	var block types.Block

	if err = block.UnmarshalSSZ(hexBytes); err != nil {
		return nil, err
	}

	return &block, nil
}

func (cfg *Cfg) FetchBlockByNumber(ctx context.Context, shardId types.ShardId, blockId transport.BlockNumber) (*types.Block, error) {
	requestBody := request{Id: 1, Jsonrpc: "2.0", Method: "debug_getBlockByNumber", Params: []any{shardId, blockId}}
	return cfg.fetchBlockData(ctx, requestBody)
}

func (cfg *Cfg) FetchBlockByHash(ctx context.Context, shardId types.ShardId, blockHash common.Hash) (*types.Block, error) {
	requestBody := request{Id: 1, Jsonrpc: "2.0", Method: "debug_getBlockByHash", Params: []any{shardId, blockHash}}
	return cfg.fetchBlockData(ctx, requestBody)
}

func (cfg *Cfg) FetchLastBlock(ctx context.Context, shardId types.ShardId) (*types.Block, error) {
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

	log.Info().Msgf("Response body: %s", body)

	var bodyResponse blockShardIdsResponse

	if err = json.Unmarshal(body, &bodyResponse); err != nil {
		return nil, err
	}

	if bodyResponse.Result == nil {
		return nil, errors.New("shards not found")
	}

	return bodyResponse.Result, nil
}
