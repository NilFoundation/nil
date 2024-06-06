package block

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/NilFoundation/nil/client/mock"
	"github.com/NilFoundation/nil/core/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock response data for a successful block fetch
var mockSuccessResponse = map[string]interface{}{
	"hash":       "0x294a68120c056a549d314efa8306dafdb856f7b51dde976df0e807e001ff84ac",
	"messages":   []interface{}{},
	"number":     "0xf",
	"parentHash": "0x15dd3170e2e6a80d41fe81977c6d08940c32834356b086eada2bae57e6bbd20f",
	"receipts":   []interface{}{},
	"shardId":    0,
}

// TestFetchBlock_Successfully tests fetching a block without errors
func TestFetchBlock_Successfully(t *testing.T) {
	t.Parallel()

	mockClient := &mock.MockClient{
		CallFn: func(method string, params []interface{}) (json.RawMessage, error) {
			mockResponse, err := json.Marshal(mockSuccessResponse)
			if err != nil {
				return nil, err
			}
			return mockResponse, nil
		},
	}

	// Initialize the service with the mock client
	service := NewService(mockClient, types.BaseShardId)

	// Call the FetchBlockByHash
	response, err := service.FetchBlockByHash("")
	require.NoError(t, err)

	// Check if the response matches the expected mock response
	expectedResponse, err := json.MarshalIndent(mockSuccessResponse, "", "  ")
	require.NoError(t, err)
	require.JSONEq(t, string(expectedResponse), string(response))

	// Call the FetchBlockByNumber
	response, err = service.FetchBlockByNumber("")
	require.NoError(t, err)

	// Check if the response matches the expected mock response
	expectedResponse, err = json.MarshalIndent(mockSuccessResponse, "", "  ")
	require.NoError(t, err)
	assert.JSONEq(t, string(expectedResponse), string(response))

	// Call the fetchBlock
	response, err = service.fetchBlock("", "")
	require.NoError(t, err)

	// Check if the response matches the expected mock response
	expectedResponse, err = json.MarshalIndent(mockSuccessResponse, "", "  ")
	require.NoError(t, err)
	assert.JSONEq(t, string(expectedResponse), string(response))
}

// TestFetchBlock_Err tests error scenarios for fetching a block
func TestFetchBlock_Err(t *testing.T) {
	t.Parallel()

	// Test case for RPC error
	mockClient := &mock.MockClient{
		CallFn: func(method string, params []interface{}) (json.RawMessage, error) {
			return nil, errors.New("RPC error")
		},
	}

	// Initialize the service with the mock client
	service := NewService(mockClient, types.BaseShardId)

	// Call the fetchBlock
	_, err := service.FetchBlockByHash("0x294a68120c056a549d314efa8306dafdb856f7b51dde976df0e807e001ff84ac")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "RPC error")

	// Test case for JSON unmarshal error
	mockClient = &mock.MockClient{
		CallFn: func(method string, params []interface{}) (json.RawMessage, error) {
			invalidJSON := []byte(`{"hash": "0x294a68120c056a549d314efa8306dafdb856f7b51dde976df0e807e001ff84ac"`)
			return invalidJSON, nil
		},
	}

	// Initialize the service with the mock client
	service = NewService(mockClient, types.BaseShardId)

	// Call the fetchBlock
	_, err = service.FetchBlockByHash("0x294a68120c056a549d314efa8306dafdb856f7b51dde976df0e807e001ff84ac")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected end of JSON input")
}
