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

// Mock response data for a successful receipt fetch
var mockSuccessReceiptResponse = &jsonrpc.RPCReceipt{
	BlockHash:       common.EmptyHash,
	BlockNumber:     0,
	Bloom:           hexutil.Bytes([]byte("bloom")),
	ContractAddress: types.HexToAddress("0x0000f3306bba76b1e215bc3c78c859396786f415"),
	GasUsed:         127,
	Logs:            []*jsonrpc.RPCLog{},
	MsgHash:         common.HexToHash("0x1dbf1be14f8329839e81695a6c656c4f76d0e1da79ba6e67881e16db16981d61"),
	MsgIndex:        0,
	Success:         true,
}

// TestFetchReceipt_Successfully tests fetching a receipt without errors
func TestFetchReceipt_Successfully(t *testing.T) {
	t.Parallel()

	mockClient := &mock.MockClient{
		Receipt: mockSuccessReceiptResponse,
	}

	// Initialize the service with the mock client
	service := NewService(mockClient, nil)

	// Call the FetchReceiptByHash
	response, err := service.FetchReceiptByHash(types.BaseShardId, "0x1234")
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
		Err: errors.New("RPC error"),
	}

	// Initialize the service with the mock client
	service := NewService(mockClient, nil)

	// Call the FetchReceiptByHash
	_, err := service.FetchReceiptByHash(types.BaseShardId, "0x1234")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "RPC error")
}
