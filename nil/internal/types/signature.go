package types

import (
	"encoding/hex"

	"github.com/NilFoundation/nil/nil/common/hexutil"
)

type Signature []byte

func (s Signature) MarshalText() ([]byte, error) {
	return hexutil.Bytes(s[:]).MarshalText()
}

func (s *Signature) UnmarshalText(input []byte) error {
	var b hexutil.Bytes
	if err := b.UnmarshalText(input); err != nil {
		return err
	}
	copy(*s, b[:])
	return nil
}

func (s Signature) Hex() string {
	enc := make([]byte, hex.EncodedLen(len(s[:]))+2)
	copy(enc, "0x")
	hex.Encode(enc[2:], s[:])
	return string(enc)
}

func (s Signature) String() string {
	return s.Hex()
}
