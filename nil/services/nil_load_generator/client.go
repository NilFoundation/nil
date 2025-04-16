package nil_load_generator

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync/atomic"

	"github.com/NilFoundation/nil/nil/client/rpc"
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/version"
	"github.com/NilFoundation/nil/nil/internal/types"
)

type Client struct {
	requestId  atomic.Uint64
	endpoint   string
	httpClient http.Client
}

func NewClient(endpoint string) *Client {
	httpc, endpoint := rpc.NewHttpClient(endpoint)
	return &Client{endpoint: endpoint, httpClient: httpc}
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

	body, err := rpc.SendRequest(ctx, c.httpClient, c.endpoint, requestBody, map[string]string{
		"User-Agent": "nilloadgen/" + version.GetGitRevCount(),
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

func (c *Client) GetHealthCheck(ctx context.Context) (bool, error) {
	response, err := c.sendRequest(ctx, "nilloadgen_healthCheck", []any{})
	if err != nil {
		return false, err
	}
	var res bool
	if err := json.Unmarshal(response, &res); err != nil {
		return false, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	return res, nil
}

func (c *Client) GetSmartAccountsAddr(ctx context.Context) ([]types.Address, error) {
	response, err := c.sendRequest(ctx, "nilloadgen_smartAccountsAddr", []any{})
	if err != nil {
		return nil, err
	}
	var res []types.Address
	if err := json.Unmarshal(response, &res); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	return res, nil
}

func (c *Client) CallSwap(
	ctx context.Context, pairShard types.ShardId, amountOut1, amountOut2, swapAmount uint64,
) (common.Hash, error) {
	response, err := c.sendRequest(ctx, "nilloadgen_callSwap", []any{pairShard, amountOut1, amountOut2, swapAmount})
	if err != nil {
		return common.EmptyHash, err
	}
	var res common.Hash
	if err := json.Unmarshal(response, &res); err != nil {
		return common.EmptyHash, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	return res, nil
}
