package crypto

import (
	"github.com/holiman/uint256"
)

// See Appendix F "Signing Transactions" of the Yellow Paper
func TransactionSignatureIsValid(v byte, r, s *uint256.Int) bool {
	if r.IsZero() || s.IsZero() {
		return false
	}

	return r.Lt(secp256k1N) && s.Lt(secp256k1N) && (v == 0 || v == 1)
}

func TransactionSignatureIsValidBytes(sign []byte) bool {
	if len(sign) != 65 {
		return false
	}

	var r, s uint256.Int
	r.SetBytes(sign[:32])
	s.SetBytes(sign[32:64])

	v := sign[64]
	return TransactionSignatureIsValid(v, &r, &s)
}
