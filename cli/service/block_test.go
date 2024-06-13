package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/NilFoundation/nil/client/mock"
	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/rpc/jsonrpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock response data for a successful block fetch
var mockBlockResponse = &jsonrpc.RPCBlock{
	Hash:       common.HexToHash("294a68120c056a549d314efa8306dafdb856f7b51dde976df0e807e001ff84ac"),
	ParentHash: common.HexToHash("15dd3170e2e6a80d41fe81977c6d08940c32834356b086eada2bae57e6bbd20f"),
	Messages:   []any{},
	Number:     mockBlockNumber,
	ShardId:    types.BaseShardId,
}

const mockBlockNumber = types.BlockNumber(0xf)

// TestFetchBlock_Successfully tests fetching a block without errors
func TestFetchBlock_Successfully(t *testing.T) {
	t.Parallel()

	mockClient := &mock.MockClient{
		Block: mockBlockResponse,
	}

	// Initialize the service with the mock client
	service := NewService(mockClient, "", types.BaseShardId)

	// Call the FetchBlockByHash
	response, err := service.FetchBlockByHash(mockBlockResponse.Hash.Hex())
	require.NoError(t, err)

	// Check if the response matches the expected mock response
	expectedResponse, err := json.MarshalIndent(mockBlockResponse, "", "  ")
	require.NoError(t, err)
	require.JSONEq(t, string(expectedResponse), string(response))

	// Call the FetchBlockByNumber
	response, err = service.FetchBlockByNumber(fmt.Sprintf("%x", mockBlockNumber))
	require.NoError(t, err)

	// Check if the response matches the expected mock response
	expectedResponse, err = json.MarshalIndent(mockBlockResponse, "", "  ")
	require.NoError(t, err)
	assert.JSONEq(t, string(expectedResponse), string(response))

	// Check if the response matches the expected mock response
	expectedResponse, err = json.MarshalIndent(mockBlockResponse, "", "  ")
	require.NoError(t, err)
	assert.JSONEq(t, string(expectedResponse), string(response))
}

// TestFetchBlock_Err tests error scenarios for fetching a block
func TestFetchBlock_Err(t *testing.T) {
	t.Parallel()

	// Test case for RPC error
	mockClient := &mock.MockClient{
		Err: errors.New("RPC error"),
	}

	// Initialize the service with the mock client
	service := NewService(mockClient, "", types.BaseShardId)

	// Call the fetchBlock
	_, err := service.FetchBlockByHash("0x294a68120c056a549d314efa8306dafdb856f7b51dde976df0e807e001ff84ac")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "RPC error")
}
