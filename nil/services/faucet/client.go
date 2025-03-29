package faucet

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
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

func (c *Client) sendRequest(method string, params []any) (json.RawMessage, error) {
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
	req, err := http.NewRequest(http.MethodPost, c.endpoint, bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "faucet/"+version.GetGitRevCount())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, body)
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
	faucetAddress types.Address,
	contractAddressTo types.Address,
	amount types.Value,
) (common.Hash, error) {
	response, err := c.sendRequest("faucet_topUpViaFaucet", []any{faucetAddress, contractAddressTo, amount})
	if err != nil {
		return common.EmptyHash, err
	}
	var hash common.Hash
	if err := json.Unmarshal(response, &hash); err != nil {
		return common.EmptyHash, err
	}
	return hash, nil
}

func (c *Client) GetFaucets() (map[string]types.Address, error) {
	faucets := make(map[string]types.Address)
	response, err := c.sendRequest("faucet_getFaucets", []any{})
	if err != nil {
		return faucets, err
	}
	if err := json.Unmarshal(response, &faucets); err != nil {
		return nil, fmt.Errorf("failed to unmarshal contract: %w", err)
	}
	return faucets, nil
}
