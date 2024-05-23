package crypto

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"

	"github.com/NilFoundation/nil/common/hexutil"
	"github.com/NilFoundation/nil/common/math"
	"github.com/holiman/uint256"
	"github.com/iden3/go-iden3-crypto/poseidon"
)

// DigestLength sets the signature digest exact length
const DigestLength = 32

var (
	secp256k1N     = new(uint256.Int).SetBytes(hexutil.MustDecodeHex("0xfffffffffffffffffffffffffffffffebaaedce6af48a03bbfd25e8cd0364141"))
	secp256k1NBig  = secp256k1N.ToBig()
	secp256k1halfN = new(uint256.Int).Div(secp256k1N, uint256.NewInt(2))
)

var errInvalidPubkey = errors.New("invalid secp256k1 public key")

// ToECDSA creates a private key with the given D value.
func ToECDSA(d []byte) (*ecdsa.PrivateKey, error) {
	return toECDSA(d, true)
}

// ToECDSAUnsafe blindly converts a binary blob to a private key. It should almost
// never be used unless you are sure the input is valid and want to avoid hitting
// errors due to bad origin encoding (0 prefixes cut off).
func ToECDSAUnsafe(d []byte) *ecdsa.PrivateKey {
	priv, _ := toECDSA(d, false)
	return priv
}

// toECDSA creates a private key with the given D value. The strict parameter
// controls whether the key's length should be enforced at the curve size or
// it can also accept legacy encodings (0 prefixes).
func toECDSA(d []byte, strict bool) (*ecdsa.PrivateKey, error) {
	priv := new(ecdsa.PrivateKey)
	priv.PublicKey.Curve = S256()
	if strict && 8*len(d) != priv.Params().BitSize {
		return nil, fmt.Errorf("invalid length, need %d bits", priv.Params().BitSize)
	}
	priv.D = new(big.Int).SetBytes(d)

	// The priv.D must < N
	if priv.D.Cmp(secp256k1NBig) >= 0 {
		return nil, errors.New("invalid private key, >=N")
	}
	// The priv.D must not be zero or negative.
	if priv.D.Sign() <= 0 {
		return nil, errors.New("invalid private key, zero or negative")
	}

	priv.PublicKey.X, priv.PublicKey.Y = priv.PublicKey.Curve.ScalarBaseMult(d)
	if priv.PublicKey.X == nil {
		return nil, errors.New("invalid private key")
	}
	return priv, nil
}

// FromECDSA exports a private key into a binary dump.
func FromECDSA(priv *ecdsa.PrivateKey) []byte {
	if priv == nil {
		return nil
	}
	return math.PaddedBigBytes(priv.D, priv.Params().BitSize/8)
}

// UnmarshalPubkeyStd parses a public key from the given bytes in the standard "uncompressed" format.
// The input slice must be 65 bytes long and have this format: [4, X..., Y...]
// See MarshalPubkeyStd.
func UnmarshalPubkeyStd(pub []byte) (*ecdsa.PublicKey, error) {
	x, y := elliptic.Unmarshal(S256(), pub) //nolint
	if x == nil {
		return nil, errInvalidPubkey
	}
	return &ecdsa.PublicKey{Curve: S256(), X: x, Y: y}, nil
}

// MarshalPubkeyStd converts a public key into the standard "uncompressed" format.
// It returns a 65 bytes long slice that contains: [4, X..., Y...]
// Returns nil if the given public key is not initialized.
// See UnmarshalPubkeyStd.
func MarshalPubkeyStd(pub *ecdsa.PublicKey) []byte {
	if pub == nil || pub.X == nil || pub.Y == nil {
		return nil
	}
	return elliptic.Marshal(S256(), pub.X, pub.Y) //nolint
}

// UnmarshalPubkey parses a public key from the given bytes in the 64 bytes "uncompressed" format.
// The input slice must be 64 bytes long and have this format: [X..., Y...]
// See MarshalPubkey.
func UnmarshalPubkey(keyBytes []byte) (*ecdsa.PublicKey, error) {
	keyBytes = append([]byte{0x4}, keyBytes...)
	return UnmarshalPubkeyStd(keyBytes)
}

// MarshalPubkey converts a public key into a 64 bytes "uncompressed" format.
// It returns a 64 bytes long slice that contains: [X..., Y...]
// In the standard 65 bytes format the first byte is always constant (equal to 4),
// so it can be cut off and trivially recovered later.
// Returns nil if the given public key is not initialized.
// See UnmarshalPubkey.
func MarshalPubkey(pubkey *ecdsa.PublicKey) []byte {
	keyBytes := MarshalPubkeyStd(pubkey)
	if keyBytes == nil {
		return nil
	}
	return keyBytes[1:]
}

// HexToECDSA parses a secp256k1 private key.
func HexToECDSA(hexkey string) (*ecdsa.PrivateKey, error) {
	b, err := hex.DecodeString(hexkey)
	var byteErr hex.InvalidByteError
	if errors.As(err, &byteErr) {
		return nil, fmt.Errorf("invalid hex character %q in private key", byte(byteErr))
	} else if err != nil {
		return nil, errors.New("invalid hex data for private key")
	}
	return ToECDSA(b)
}

// GenerateKey generates a new private key.
func GenerateKey() (*ecdsa.PrivateKey, error) {
	return ecdsa.GenerateKey(S256(), rand.Reader)
}

// ValidateSignatureValues verifies whether the signature values are valid with
// the given chain rules. The v value is assumed to be either 0 or 1.
func ValidateSignatureValues(v byte, r, s *uint256.Int, homestead bool) bool {
	if r.IsZero() || s.IsZero() {
		return false
	}
	// reject upper range of s values (ECDSA malleability)
	// see discussion in secp256k1/libsecp256k1/include/secp256k1.h
	if homestead && s.Gt(secp256k1halfN) {
		return false
	}
	// Frontier: allow s to be in full N range
	return r.Lt(secp256k1N) && s.Lt(secp256k1N) && (v == 0 || v == 1)
}

func PubkeyBytesToAddress(_ uint32, pubBytes []byte) []byte {
	return poseidon.Sum(pubBytes)[12:]
}
