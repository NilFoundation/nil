package contract

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/NilFoundation/nil/client/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock response data for a successful contract operation
var mockSuccessResponse = "0x6001600101600055"

// Mock responses
var mockSuccessSeqNumResponse = json.RawMessage(`"0x1"`)

// Mock private key
var mockPrivateKey = "6c1beb99b140a104df88b6f63275feaa7fcab908b1ef0632c78539da9a486c7e"

// TestGetCode_Successfully tests getting the contract code without errors
func TestGetCode_Successfully(t *testing.T) {
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

	service := NewService(mockClient, mockPrivateKey)

	code, err := service.GetCode("0x1234")
	require.NoError(t, err)
	assert.Equal(t, mockSuccessResponse, code)
}

// TestGetCode_Err tests getting the contract code with errors
func TestGetCode_Err(t *testing.T) {
	t.Parallel()

	// Test case for RPC error
	mockClient := &mock.MockClient{
		CallFn: func(method string, params []interface{}) (json.RawMessage, error) {
			return nil, errors.New("RPC error")
		},
	}

	service := NewService(mockClient, mockPrivateKey)

	_, err := service.GetCode("0x1234")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "RPC error")

	// Test case for JSON unmarshal error
	mockClient = &mock.MockClient{
		CallFn: func(method string, params []interface{}) (json.RawMessage, error) {
			return json.RawMessage(`{"result": "0x1234567890abcdef"`), nil
		},
	}

	service = NewService(mockClient, mockPrivateKey)

	_, err = service.GetCode("0x1234")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected end of JSON input")
}

// TestRunContract_Successfully tests running a contract without errors
func TestRunContract_Successfully(t *testing.T) {
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

	service := NewService(mockClient, mockPrivateKey)

	txHash, err := service.RunContract("0x6001600101600055", "0x1234")
	require.NoError(t, err)
	assert.Equal(t, mockSuccessResponse, txHash)
}

// TestRunContract_Err tests running a contract with errors
func TestRunContract_Err(t *testing.T) {
	t.Parallel()

	// Test case for RPC error
	mockClient := &mock.MockClient{
		CallFn: func(method string, params []interface{}) (json.RawMessage, error) {
			if method == getTransactionCount {
				return nil, errors.New("RPC error")
			}
			return nil, nil
		},
	}

	service := NewService(mockClient, mockPrivateKey)

	_, err := service.RunContract("0x6001600101600055", "0x1234")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "RPC error")

	// Test case for sendRawTransactionError
	mockClient = &mock.MockClient{
		CallFn: func(method string, params []interface{}) (json.RawMessage, error) {
			if method == getTransactionCount {
				return mockSuccessSeqNumResponse, nil
			}
			if method == sendRawTransaction {
				return nil, errors.New("RPC error")
			}
			return nil, nil
		},
	}

	service = NewService(mockClient, mockPrivateKey)

	_, err = service.RunContract("0x6001600101600055", "0x1234")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "RPC error")
}

// TestDeployContract_Successfully tests deploying a contract without errors
func TestDeployContract_Successfully(t *testing.T) {
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

	service := NewService(mockClient, mockPrivateKey)

	txHash, err := service.DeployContract("0x6001600101600055")
	require.NoError(t, err)
	assert.Equal(t, mockSuccessResponse, txHash)
}

// TestContractDeployment_Err tests deploying a contract with errors
func TestContractDeployment_Err(t *testing.T) {
	t.Parallel()

	// Test case for RPC error
	mockClient := &mock.MockClient{
		CallFn: func(method string, params []interface{}) (json.RawMessage, error) {
			if method == getTransactionCount {
				return nil, errors.New("RPC error")
			}
			return nil, nil
		},
	}

	service := NewService(mockClient, mockPrivateKey)

	_, err := service.DeployContract("0x6001600101600055")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "RPC error")

	// Test case for sendRawTransactionError
	mockClient = &mock.MockClient{
		CallFn: func(method string, params []interface{}) (json.RawMessage, error) {
			if method == getTransactionCount {
				return mockSuccessSeqNumResponse, nil
			}
			if method == sendRawTransaction {
				return nil, errors.New("RPC error")
			}
			return nil, nil
		},
	}

	service = NewService(mockClient, mockPrivateKey)

	_, err = service.DeployContract("0x6001600101600055")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "RPC error")
}
