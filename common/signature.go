package common

import (
	"encoding/hex"

	"github.com/NilFoundation/nil/common/hexutil"
)

type Signature [SignatureSize]byte

var EmptySignature = Signature{}

func (s Signature) MarshalText() ([]byte, error) {
	return hexutil.Bytes(s[:]).MarshalText()
}

func (s Signature) Hex() string {
	enc := make([]byte, len(s[:])*2+2)
	copy(enc, "0x")
	hex.Encode(enc[2:], s[:])
	return string(enc)
}
