package receipt

import (
	"encoding/json"
	"errors"
	"github.com/NilFoundation/nil/core/types"
	"testing"

	"github.com/NilFoundation/nil/client/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock response data for a successful receipt fetch
var mockSuccessReceiptResponse = map[string]interface{}{
	"blockHash":       "0x0000000000000000000000000000000000000000000000000000000000000000",
	"blockNumber":     34,
	"bloom":           make([]int, 256),
	"contractAddress": "0x0000f3306bba76b1e215bc3c78c859396786f415",
	"gasUsed":         127,
	"logs":            []interface{}{},
	"messageHash":     "0x1dbf1be14f8329839e81695a6c656c4f76d0e1da79ba6e67881e16db16981d61",
	"messageIndex":    0,
	"outMsgIndex":     0,
	"success":         true,
}

// TestFetchReceipt_Successfully tests fetching a receipt without errors
func TestFetchReceipt_Successfully(t *testing.T) {
	t.Parallel()

	mockClient := &mock.MockClient{
		CallFn: func(method string, params []interface{}) (json.RawMessage, error) {
			mockResponse, err := json.Marshal(mockSuccessReceiptResponse)
			if err != nil {
				return nil, err
			}
			return mockResponse, nil
		},
	}

	// Initialize the service with the mock client
	service := NewService(mockClient, types.BaseShardId)

	// Call the FetchReceiptByHash
	response, err := service.FetchReceiptByHash("0x1234")
	require.NoError(t, err)

	// Check if the response matches the expected mock response
	expectedResponse, err := json.MarshalIndent(mockSuccessReceiptResponse, "", "  ")
	require.NoError(t, err)
	assert.JSONEq(t, string(expectedResponse), string(response))
}

// TestFetchReceipt_Err tests error scenarios for fetching a receipt
func TestFetchReceipt_Err(t *testing.T) {
	t.Parallel()

	// Test case for RPC error
	mockClient := &mock.MockClient{
		CallFn: func(method string, params []interface{}) (json.RawMessage, error) {
			return nil, errors.New("RPC error")
		},
	}

	// Initialize the service with the mock client
	service := NewService(mockClient, types.BaseShardId)

	// Call the FetchReceiptByHash
	_, err := service.FetchReceiptByHash("0x1234")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "RPC error")

	// Test case for JSON unmarshal error
	mockClient = &mock.MockClient{
		CallFn: func(method string, params []interface{}) (json.RawMessage, error) {
			invalidJSON := []byte(`{"blockHash": "0x0000000000000000000000000000000000000000000000000000000000000000"`)
			return invalidJSON, nil
		},
	}

	// Initialize the service with the mock client
	service = NewService(mockClient, types.BaseShardId)

	// Call the FetchReceiptByHash
	_, err = service.FetchReceiptByHash("0x1234")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected end of JSON input")
}
