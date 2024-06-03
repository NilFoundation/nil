package rpc

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

var (
	ErrFailedToMarshalRequest    = errors.New("failed to marshal request")
	ErrFailedToSendRequest       = errors.New("failed to send request")
	ErrUnexpectedStatusCode      = errors.New("unexpected status code")
	ErrFailedToReadResponse      = errors.New("failed to read response")
	ErrFailedToUnmarshalResponse = errors.New("failed to unmarshal response")
	ErrRPCError                  = errors.New("rpc error")
)

type RPCClient struct {
	endpoint string
}

// NewRPCClient creates a new RPCClient instance
func NewRPCClient(endpoint string) *RPCClient {
	return &RPCClient{
		endpoint: endpoint,
	}
}

// Call sends a JSON-RPC request to the server and returns the response
func (c *RPCClient) Call(method string, params []interface{}) (json.RawMessage, error) {
	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  method,
		"params":  params,
		"id":      1,
	}

	requestBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrFailedToMarshalRequest, err)
	}

	resp, err := http.Post(c.endpoint, "application/json", bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrFailedToSendRequest, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: %d", ErrUnexpectedStatusCode, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrFailedToReadResponse, err)
	}

	var rpcResponse map[string]json.RawMessage
	if err := json.Unmarshal(body, &rpcResponse); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrFailedToUnmarshalResponse, err)
	}

	if errorMsg, ok := rpcResponse["error"]; ok {
		return nil, fmt.Errorf("%w: %s", ErrRPCError, errorMsg)
	}

	return rpcResponse["result"], nil
}
