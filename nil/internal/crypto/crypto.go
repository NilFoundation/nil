package crypto

import (
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/hex"

	gethcrypto "github.com/ethereum/go-ethereum/crypto"
)

// PrivateKeyToEthereumFormat formats the private key in Ethereum format (hexadecimal)
func PrivateKeyToEthereumFormat(priv *ecdsa.PrivateKey) string {
	return hex.EncodeToString(gethcrypto.FromECDSA(priv))
}

func GenerateKeyPair() (*ecdsa.PrivateKey, []byte, error) {
	privateKey, err := ecdsa.GenerateKey(gethcrypto.S256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	publicKey := gethcrypto.FromECDSAPub(&privateKey.PublicKey)
	return privateKey, publicKey, err
}
