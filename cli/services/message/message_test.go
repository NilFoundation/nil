package message_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/NilFoundation/nil/cli/services/message"
	"github.com/NilFoundation/nil/client/mock"
	"github.com/NilFoundation/nil/core/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock response data for a successful message fetch
var mockSuccessMessageResponse = map[string]interface{}{
	"data":      "AAAAAAAAAAAAAAAAGAAAABgAAAAYAAAAYIBgQFI0gBVgDldfgP1bUGEBgYBhABxfOV/z/mCAYEBSNIAVYQAPV1+A/VtQYAQ2EGEANFdfNWDgHIBjIJZSVRRhADhXgGPQneCKFGEAVldbX4D9W2EAQGEAYFZbYEBRYQBNkZBhANJWW2BAUYCRA5DzW2EAXmEAaFZbAFtfgFSQUJBWW2ABX4CCglRhAHmRkGEBGFZbklBQgZBVUH+T/m05fHT98UAqi3Lke2hRLwUQ17mKS8TL32rHEIs8WV9UYEBRYQCwkZBhANJWW2BAUYCRA5ChVltfgZBQkZBQVlthAMyBYQC6VluCUlBQVltfYCCCAZBQYQDlX4MBhGEAw1ZbkpFQUFZbf05Ie3EAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAX1JgEWAEUmAkX/1bX2EBIoJhALpWW5FQYQEtg2EAulZbklCCggGQUICCERVhAUVXYQFEYQDrVltbkpFQUFb+omRpcGZzWCISIKwwh/vpt0MS5wEqsie5y/rN3AelCWvUAXRIgGE519yjZHNvbGNDAAgZADM=",
	"from":      "0x0000f9172ae011e8802e295de907a7e039d91da5",
	"gasLimit":  "0",
	"gasPrice":  "0",
	"signature": "0x68c0973bebacf32ae1ef6c8147e6301f103fc7d6125dc2444ed3f425f73ea11e28dafd13a5008bd6b692545a85eb54d453c9c1705e1bd8af71453a1c23f48ddc00",
	"to":        "0x0000000000000000000000000000000000000000",
	"value":     "0",
}

// TestFetchMessage_Successfully tests fetching a message without errors
func TestFetchMessage_Successfully(t *testing.T) {
	t.Parallel()

	mockClient := &mock.MockClient{
		CallFn: func(method string, params []interface{}) (json.RawMessage, error) {
			mockResponse, err := json.Marshal(mockSuccessMessageResponse)
			if err != nil {
				return nil, err
			}
			return mockResponse, nil
		},
	}

	// Initialize the service with the mock client
	service := message.NewService(mockClient, types.BaseShardId)

	// Call the FetchMessageByHash
	response, err := service.FetchMessageByHash("0x1234")
	require.NoError(t, err)

	// Check if the response matches the expected mock response
	expectedResponse, err := json.MarshalIndent(mockSuccessMessageResponse, "", "  ")
	require.NoError(t, err)
	assert.JSONEq(t, string(expectedResponse), string(response))
}

// TestFetchMessage_Err tests error scenarios for fetching a message
func TestFetchMessage_Err(t *testing.T) {
	t.Parallel()

	// Test case for RPC error
	mockClient := &mock.MockClient{
		CallFn: func(method string, params []interface{}) (json.RawMessage, error) {
			return nil, errors.New("RPC error")
		},
	}

	// Initialize the service with the mock client
	service := message.NewService(mockClient, types.BaseShardId)

	// Call the FetchMessageByHash
	_, err := service.FetchMessageByHash("0x1234")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "RPC error")

	// Test case for JSON unmarshal error
	mockClient = &mock.MockClient{
		CallFn: func(method string, params []interface{}) (json.RawMessage, error) {
			invalidJSON := []byte(`{"data": "invalid"`)
			return invalidJSON, nil
		},
	}

	// Initialize the service with the mock client
	service = message.NewService(mockClient, types.BaseShardId)

	// Call the FetchMessageByHash
	_, err = service.FetchMessageByHash("0x1234")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected end of JSON input")
}
