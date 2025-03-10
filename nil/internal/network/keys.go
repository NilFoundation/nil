package network

import (
	"crypto/rand"
	"errors"

	"github.com/NilFoundation/nil/nil/internal/network/internal"
	libp2pcrypto "github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
)

// PrivateKey is the type of the network private key.
// We can switch to ecdsa.PrivateKey later if needed.
// Example: https://github.com/prysmaticlabs/prysm/blob/develop/crypto/ecdsa/utils.go.
type PrivateKey = libp2pcrypto.PrivKey

// LoadOrGenerateKeys loads the network keys from the file if it exists,
// otherwise generates new keys and saves them to the file.
// If the file exists but the keys are invalid, an error is returned.
func LoadOrGenerateKeys(fileName string) (PrivateKey, error) {
	key, err := internal.LoadOrGenerateKey(fileName, internal.Libp2pKeyManager{})
	if err != nil {
		return nil, err
	}
	libp2pKey, ok := key.(internal.Libp2pKey)
	if !ok {
		return nil, errors.New("failed to assert type to internal.Libp2pKey")
	}
	internal.Logger.Info().Msgf("Loaded network keys from %s", fileName)
	return libp2pKey.PrivKey, nil
}

// GeneratePrivateKey generates a new ECDSA private key with the secp256k1 curve.
// ecdsa package is not used because secp256k1 is not supported by the x509 package.
// (x509 is used by the standard library to encode and decode keys.)
// libp2p provides its own (un)marshaling functions for secp256k1 keys.
func GeneratePrivateKey() (libp2pcrypto.PrivKey, error) {
	res, _, err := libp2pcrypto.GenerateSecp256k1Key(rand.Reader)
	return res, err
}

func SerializeKeys(privKey libp2pcrypto.PrivKey) ([]byte, []byte, peer.ID, error) {
	return internal.Libp2pKey{PrivKey: privKey}.SerializeKeys()
}
