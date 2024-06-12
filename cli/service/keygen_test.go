package service

import (
	"testing"

	"github.com/NilFoundation/nil/client/mock"
	"github.com/NilFoundation/nil/core/types"
	"github.com/stretchr/testify/require"
)

// TestGenerateNewKey verifies that the GenerateNewKey function generates a new private key
func TestGenerateNewKey(t *testing.T) {
	t.Parallel()

	keyManager := NewService(&mock.MockClient{}, "", types.BaseShardId)

	err := keyManager.GenerateNewKey()
	require.NoError(t, err, "should generate a new key without error")
	require.NotNil(t, keyManager.privateKey, "private key should not be nil")
}

// TestGenerateKeyFromHex checks that a hexadecimal key string can be correctly parsed into an ECDSA private key
func TestGenerateKeyFromHex(t *testing.T) {
	t.Parallel()

	keyManager := NewService(&mock.MockClient{}, "", types.BaseShardId)

	// Define a hexadecimal key string
	hexKey := "bc1416ea4d826a6e134f1f9f6f966ff387eb13d94df09e88be505f1412527115"

	err := keyManager.GenerateKeyFromHex(hexKey)
	require.NoError(t, err, "should parse hex to ECDSA without error")
	require.NotNil(t, keyManager.privateKey, "private key should not be nil")
}

// TestGetAddressAndKey tests the GetAddressAndKey function
func TestGetAddressAndKey(t *testing.T) {
	t.Parallel()

	keyManager := NewService(&mock.MockClient{}, "", types.BaseShardId)

	// Define the hexadecimal key and expected address
	hexKey := "bc1416ea4d826a6e134f1f9f6f966ff387eb13d94df09e88be505f1412527115"

	// Generate key from the provided hexadecimal key
	err := keyManager.GenerateKeyFromHex(hexKey)
	require.NoError(t, err, "should generate a key from hex without error")

	// Get the address and key
	privHex := keyManager.GetPrivateKey()

	// Verify the results
	require.Equal(t, hexKey, privHex, "private key hex should match expected")
}
