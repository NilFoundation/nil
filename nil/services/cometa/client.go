package cometa

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync/atomic"

	"github.com/NilFoundation/nil/nil/common/version"
	"github.com/NilFoundation/nil/nil/internal/types"
)

type Client struct {
	requestId  atomic.Uint64
	endpoint   string
	httpClient http.Client
}

func NewClient(endpoint string) *Client {
	c := &Client{}
	switch {
	case strings.HasPrefix(endpoint, "unix://"):
		socketPath := strings.TrimPrefix(endpoint, "unix://")
		if socketPath == "" {
			return nil
		}
		c.endpoint = "http://unix"
		c.httpClient = http.Client{
			Transport: &http.Transport{
				DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
					return net.Dial("unix", socketPath)
				},
			},
		}
	case strings.HasPrefix(endpoint, "tcp://"):
		c.endpoint = "http://" + strings.TrimPrefix(endpoint, "tcp://")
	default:
		c.endpoint = endpoint
	}
	return c
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
	req.Header.Set("User-Agent", "cometa/"+version.GetGitRevision())

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

func (c *Client) GetContract(address types.Address) (*ContractData, error) {
	response, err := c.sendRequest("cometa_getContract", []any{address})
	if err != nil {
		return nil, err
	}
	var contract ContractData
	if err := json.Unmarshal(response, &contract); err != nil {
		return nil, fmt.Errorf("failed to unmarshal contract: %w", err)
	}
	return &contract, nil
}

func (c *Client) GetLocation(address types.Address, pc uint64) (*Location, error) {
	response, err := c.sendRequest("cometa_getLocation", []any{address, pc})
	if err != nil {
		return nil, err
	}
	var loc Location
	if err := json.Unmarshal(response, &loc); err != nil {
		return nil, fmt.Errorf("failed to unmarshal contract: %w", err)
	}
	return &loc, nil
}

func (c *Client) CompileContract(inputJson string) (*ContractData, error) {
	response, err := c.sendRequest("cometa_compileContract", []any{inputJson})
	if err != nil {
		return nil, err
	}
	var contractData ContractData
	if err := json.Unmarshal(response, &contractData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal contract: %w", err)
	}
	return &contractData, nil
}

func (c *Client) RegisterContract(contractData *ContractData, address types.Address) error {
	_, err := c.sendRequest("cometa_registerContract", []any{contractData, address})
	if err != nil {
		return err
	}
	return nil
}
