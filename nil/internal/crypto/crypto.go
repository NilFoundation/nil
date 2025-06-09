package crypto

import (
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/hex"

	gethcommon "github.com/ethereum/go-ethereum/common"
	gethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"
)

var secp256k1N = new(uint256.Int).SetBytes(gethcommon.FromHex(
	"0xfffffffffffffffffffffffffffffffebaaedce6af48a03bbfd25e8cd0364141"))

// PrivateKeyToEthereumFormat formats the private key in Ethereum format (hexadecimal)
func PrivateKeyToEthereumFormat(priv *ecdsa.PrivateKey) string {
	return hex.EncodeToString(gethcrypto.FromECDSA(priv))
}

func GenerateKeyPair() (*ecdsa.PrivateKey, []byte, error) {
	privateKey, err := ecdsa.GenerateKey(gethcrypto.S256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	publicKey := gethcrypto.CompressPubkey(&privateKey.PublicKey)
	return privateKey, publicKey, err
}
