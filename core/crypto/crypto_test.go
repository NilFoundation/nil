package crypto

import (
	"crypto/ecdsa"
	"encoding/hex"
	"errors"
	"reflect"
	"testing"

	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

var (
	testPrivHex   = "289c2857d4598e37fb9647507e47a309d6133539bf21a8b9cb6df88fd5232032"
	testPubkeyHex = "7db227d7094ce215c3a0f57e1bcc732551fe351f94249471934567e0f5dc1bf795962b8cccb87a2eb56b29fbe37d614e2f4c3c45b789ae4f1f51f4cb21972ffd"
)

// These tests are sanity checks.
// They should ensure that we don't e.g. use Sha3-224 instead of Sha3-256
// and that the sha3 library uses keccak-f permutation.
func TestToECDSAErrors(t *testing.T) {
	t.Parallel()

	if _, err := HexToECDSA("0000000000000000000000000000000000000000000000000000000000000000"); err == nil {
		t.Fatal("HexToECDSA should've returned error")
	}
	if _, err := HexToECDSA("ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"); err == nil {
		t.Fatal("HexToECDSA should've returned error")
	}
}

func TestUnmarshalPubkey(t *testing.T) {
	t.Parallel()

	key, err := UnmarshalPubkey(nil)
	if !errors.Is(err, errInvalidPubkey) || key != nil {
		t.Fatalf("expected error, got %v, %v", err, key)
	}
	key, err = UnmarshalPubkey([]byte{1, 2, 3})
	if !errors.Is(err, errInvalidPubkey) || key != nil {
		t.Fatalf("expected error, got %v, %v", err, key)
	}

	x, err := uint256.FromHex("0x760c4460e5336ac9bbd87952a3c7ec4363fc0a97bd31c86430806e287b437fd1")
	require.NoError(t, err)
	y, err := uint256.FromHex("0xb01abc6e1db640cf3106b520344af1d58b00b57823db3e1407cbc433e1b6d04d")
	require.NoError(t, err)

	var (
		enc, _ = hex.DecodeString("760c4460e5336ac9bbd87952a3c7ec4363fc0a97bd31c86430806e287b437fd1b01abc6e1db640cf3106b520344af1d58b00b57823db3e1407cbc433e1b6d04d")
		dec    = &ecdsa.PublicKey{
			Curve: S256(),
			X:     x.ToBig(),
			Y:     y.ToBig(),
		}
	)
	key, err = UnmarshalPubkey(enc)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !reflect.DeepEqual(key, dec) {
		t.Fatal("wrong result")
	}
}

func TestMarshalPubkey(t *testing.T) {
	t.Parallel()

	check := func(privateKeyHex, expectedPubkeyHex string) {
		key, err := HexToECDSA(privateKeyHex)
		if err != nil {
			t.Errorf("bad private key: %s", err)
			return
		}
		pubkeyHex := hex.EncodeToString(MarshalPubkey(&key.PublicKey))
		if pubkeyHex != expectedPubkeyHex {
			t.Errorf("unexpected public key: %s", pubkeyHex)
		}
	}

	check(testPrivHex, testPubkeyHex)
	check(
		"36a7edad64d51a568b00e51d3fa8cd340aa704153010edf7f55ab3066ca4ef21",
		"24bfa2cdce7c6a41184fa0809ad8d76969b7280952e9aa46179d90cfbab90f7d2b004928f0364389a1aa8d5166281f2ff7568493c1f719e8f6148ef8cf8af42d",
	)
}

func TestInvalidSign(t *testing.T) {
	t.Parallel()

	if _, err := Sign(make([]byte, 1), nil); err == nil {
		t.Errorf("expected sign with hash 1 byte to error")
	}
	if _, err := Sign(make([]byte, 33), nil); err == nil {
		t.Errorf("expected sign with hash 33 byte to error")
	}
}

func TestValidateSignatureValues(t *testing.T) {
	t.Parallel()

	check := func(expected bool, v byte, r, s *uint256.Int) {
		if ValidateSignatureValues(v, r, s, false) != expected {
			t.Errorf("mismatch for v: %d r: %d s: %d want: %v", v, r, s, expected)
		}
	}
	minusOne := uint256.NewInt(0).SetAllOne()
	one := uint256.NewInt(1)
	zero := uint256.NewInt(0)
	secp256k1nMinus1 := new(uint256.Int).Sub(secp256k1N, one)

	// correct v,r,s
	check(true, 0, one, one)
	check(true, 1, one, one)
	// incorrect v, correct r,s,
	check(false, 2, one, one)
	check(false, 3, one, one)

	// incorrect v, combinations of incorrect/correct r,s at lower limit
	check(false, 2, zero, zero)
	check(false, 2, zero, one)
	check(false, 2, one, zero)
	check(false, 2, one, one)

	// correct v for any combination of incorrect r,s
	check(false, 0, zero, zero)
	check(false, 0, zero, one)
	check(false, 0, one, zero)

	check(false, 1, zero, zero)
	check(false, 1, zero, one)
	check(false, 1, one, zero)

	// correct sig with max r,s
	check(true, 0, secp256k1nMinus1, secp256k1nMinus1)
	// correct v, combinations of incorrect r,s at upper limit
	check(false, 0, secp256k1N, secp256k1nMinus1)
	check(false, 0, secp256k1nMinus1, secp256k1N)
	check(false, 0, secp256k1N, secp256k1N)

	// current callers ensures r,s cannot be negative, but let's test for that too
	// as crypto package could be used stand-alone
	check(false, 0, minusOne, one)
	check(false, 0, one, minusOne)
}
