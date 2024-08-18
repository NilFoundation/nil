package network

import "errors"

var (
	// Errors that happen during network initialization.

	// ErrNetworkDisabled is returned when the network is disabled in the config.
	ErrNetworkDisabled = errors.New("network is disabled in config")
	// ErrPrivateKeyMissing is returned when the private key is missing in the config.
	ErrPrivateKeyMissing = errors.New("private key is missing in config")
	// ErrPublicKeyMismatch is returned when the public key does not match the private key.
	ErrPublicKeyMismatch = errors.New("public key does not match the private key")
	// ErrIdentityMismatch is returned when the identity does not match the public key.
	ErrIdentityMismatch = errors.New("identity does not match the public key")
)
