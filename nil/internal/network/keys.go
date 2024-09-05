package network

import (
	"fmt"
	"os"

	"github.com/NilFoundation/nil/nil/internal/network/internal"
	libp2pcrypto "github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
)

// PrivateKey is the type of the network private key.
// We can switch to ecdsa.PrivateKey later if needed.
// Example: https://github.com/prysmaticlabs/prysm/blob/develop/crypto/ecdsa/utils.go.
type PrivateKey = libp2pcrypto.PrivKey

// LoadOrGenerateKeys loads the keys from the file if it exists,
// otherwise generates new keys and saves them to the file.
// If the file exists but the keys are invalid, an error is returned.
func LoadOrGenerateKeys(fileName string) (PrivateKey, error) {
	_, err := os.Stat(fileName)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}

		return GenerateAndDumpKeys(fileName)
	}

	privKey, pubKey, id, err := internal.LoadKeys(fileName)
	if err != nil {
		return nil, fmt.Errorf("failed to load keys: %w", err)
	}

	if !privKey.GetPublic().Equals(pubKey) {
		return nil, ErrPublicKeyMismatch
	}

	identity, err := peer.IDFromPublicKey(pubKey)
	if err != nil {
		return nil, err
	}
	if id != identity {
		return nil, ErrIdentityMismatch
	}

	internal.Logger.Info().Msgf("Loaded network keys from %s", fileName)

	return privKey, nil
}

func GenerateAndDumpKeys(fileName string) (PrivateKey, error) {
	privKey, err := internal.GeneratePrivateKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate keys: %w", err)
	}

	if err := internal.DumpKeys(privKey, fileName); err != nil {
		return nil, fmt.Errorf("failed to save keys: %w", err)
	}

	internal.Logger.Info().Msgf("Saved network keys to %s", fileName)

	return privKey, nil
}
