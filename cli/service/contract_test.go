package service

import (
	"errors"
	"testing"

	"github.com/NilFoundation/nil/client/mock"
	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/hexutil"
	"github.com/NilFoundation/nil/core/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock response data for a successful contract operation
var mockSuccessResponse = "0x6001600101600055"

// Mock responses
var mockSuccessSeqNumResponse = types.Seqno(0x1)

// Mock hash
var mockSuccessHash = common.HexToHash("294a68120c056a549d314efa8306dafdb856f7b51dde976df0e807e001ff84ac")

// Mock private key
var mockPrivateKey = "6c1beb99b140a104df88b6f63275feaa7fcab908b1ef0632c78539da9a486c7e"

// TestGetCode_Successfully tests getting the contract code without errors
func TestGetCode_Successfully(t *testing.T) {
	t.Parallel()

	code := types.Code(hexutil.MustDecodeHex(mockSuccessResponse))
	mockClient := &mock.MockClient{
		Code: &code,
	}

	service := NewService(mockClient, mockPrivateKey, types.BaseShardId)

	codeHex, err := service.GetCode("0x1234")
	require.NoError(t, err)
	assert.Equal(t, mockSuccessResponse, codeHex)
}

// TestGetCode_Err tests getting the contract code with errors
func TestGetCode_Err(t *testing.T) {
	t.Parallel()

	// Test case for RPC error
	mockClient := &mock.MockClient{
		Err: errors.New("RPC error"),
	}

	service := NewService(mockClient, mockPrivateKey, types.BaseShardId)

	_, err := service.GetCode("0x1234")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "RPC error")
}

// TestRunContract_Successfully tests running a contract without errors
func TestRunContract_Successfully(t *testing.T) {
	t.Parallel()

	mockClient := &mock.MockClient{
		Seqno: &mockSuccessSeqNumResponse,
		Hash:  &mockSuccessHash,
	}

	service := NewService(mockClient, mockPrivateKey, types.BaseShardId)

	txHash, err := service.RunContract(types.EmptyAddress.Hex(), "0x6001600101600055", "0x1234")
	require.NoError(t, err)
	assert.Equal(t, mockSuccessHash, common.HexToHash(txHash))
}

// TestRunContract_Err tests running a contract with errors
func TestRunContract_Err(t *testing.T) {
	t.Parallel()

	// Test case for RPC error
	mockClient := &mock.MockClient{
		Err: errors.New("RPC error"),
	}

	service := NewService(mockClient, mockPrivateKey, types.BaseShardId)

	_, err := service.RunContract(types.EmptyAddress.Hex(), "0x6001600101600055", "0x1234")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "RPC error")
}

// TestDeployContract_Successfully tests deploying a contract without errors
func TestDeployContract_Successfully(t *testing.T) {
	t.Parallel()

	mockClient := &mock.MockClient{
		Seqno: &mockSuccessSeqNumResponse,
		Hash:  &mockSuccessHash,
	}

	service := NewService(mockClient, mockPrivateKey, types.BaseShardId)

	txHash, _, err := service.DeployContract(types.EmptyAddress.Hex(), "0x6001600101600055")
	require.NoError(t, err)
	assert.Equal(t, mockSuccessHash, common.HexToHash(txHash))
}

// TestContractDeployment_Err tests deploying a contract with errors
func TestContractDeployment_Err(t *testing.T) {
	t.Parallel()

	// Test case for RPC error
	mockClient := &mock.MockClient{
		Err: errors.New("RPC error"),
	}

	service := NewService(mockClient, mockPrivateKey, types.BaseShardId)

	_, _, err := service.DeployContract(types.EmptyAddress.Hex(), "0x6001600101600055")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "RPC error")
}
