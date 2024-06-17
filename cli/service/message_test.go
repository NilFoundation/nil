package service

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/NilFoundation/nil/client/mock"
	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/hexutil"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/rpc/jsonrpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock response data for a successful message fetch
var mockSuccessMessageResponse = &jsonrpc.RPCInMessage{
	Data:      []byte("abcd"),
	From:      types.GenerateRandomAddress(types.BaseShardId),
	To:        &types.EmptyAddress,
	Signature: types.Signature(hexutil.FromHex("0x68c0973bebacf32ae1ef6c8147e6301f103fc7d6125dc2444ed3f425f73ea11e28dafd13a5008bd6b692545a85eb54d453c9c1705e1bd8af71453a1c23f48ddc00")),
}

// TestFetchMessage_Successfully tests fetching a message without errors
func TestFetchMessage_Successfully(t *testing.T) {
	t.Parallel()

	mockClient := &mock.MockClient{
		InMessage: mockSuccessMessageResponse,
	}

	// Initialize the service with the mock client
	service := NewService(mockClient, nil)

	// Call the FetchMessageByHash
	response, err := service.FetchMessageByHash(types.BaseShardId, common.EmptyHash)
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
		Err: errors.New("RPC error"),
	}

	// Initialize the service with the mock client
	service := NewService(mockClient, nil)

	// Call the FetchMessageByHash
	_, err := service.FetchMessageByHash(types.BaseShardId, common.EmptyHash)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "RPC error")
}
