package faucet

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync/atomic"

	rpc_client "github.com/NilFoundation/nil/nil/client/rpc"
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/version"
	"github.com/NilFoundation/nil/nil/internal/types"
)

type Client struct {
	requestId  atomic.Uint64
	endpoint   string
	httpClient http.Client
}

func NewClient(url string) *Client {
	httpc, endpoint := rpc_client.NewHttpClient(url)
	return &Client{
		httpClient: httpc,
		endpoint:   endpoint,
	}
}

func (c *Client) IsValid() bool {
	return len(c.endpoint) > 0
}

func (c *Client) sendRequest(ctx context.Context, method string, params []any) (json.RawMessage, error) {
	request := make(map[string]any)
	request["jsonrpc"] = "2.0"
	request["method"] = method
	request["params"] = params
	request["id"] = c.requestId.Load()
	c.requestId.Add(1)

	requestBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	body, err := rpc_client.SendRequest(ctx, c.httpClient, c.endpoint, requestBody, map[string]string{
		"User-Agent": "faucet/" + version.GetGitRevCount(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	var rpcResponse map[string]json.RawMessage
	if err := json.Unmarshal(body, &rpcResponse); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	if errorMsg, ok := rpcResponse["error"]; ok {
		return nil, fmt.Errorf("rpc error: %s", errorMsg)
	}

	return rpcResponse["result"], nil
}

func (c *Client) TopUpViaFaucet(
	ctx context.Context,
	faucetAddress types.Address,
	contractAddressTo types.Address,
	amount types.Value,
) (common.Hash, error) {
	response, err := c.sendRequest(ctx, "faucet_topUpViaFaucet", []any{faucetAddress, contractAddressTo, amount})
	if err != nil {
		return common.EmptyHash, err
	}
	var hash common.Hash
	if err := json.Unmarshal(response, &hash); err != nil {
		return common.EmptyHash, err
	}
	return hash, nil
}

func (c *Client) GetFaucets(ctx context.Context) (map[string]types.Address, error) {
	faucets := make(map[string]types.Address)
	response, err := c.sendRequest(ctx, "faucet_getFaucets", []any{})
	if err != nil {
		return faucets, err
	}
	if err := json.Unmarshal(response, &faucets); err != nil {
		return nil, fmt.Errorf("failed to unmarshal contract: %w", err)
	}
	return faucets, nil
}
