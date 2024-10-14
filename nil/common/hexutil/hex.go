package hexutil

import (
	"encoding/hex"
	"math/big"

	"github.com/NilFoundation/nil/nil/common/check"
)

func MustDecodeHex(in string) []byte {
	payload, err := DecodeHex(in)
	check.PanicIfErr(err)
	return payload
}

func DecodeHex(in string) ([]byte, error) {
	in = strip0x(in)
	if len(in)%2 == 1 {
		in = "0" + in
	}
	return hex.DecodeString(in)
}

func strip0x(str string) string {
	if Has0xPrefix(str) {
		return str[2:]
	}
	return str
}

// Encode encodes b as a hex string with 0x prefix.
func Encode(b []byte) string {
	enc := make([]byte, len(b)*2+2)
	copy(enc, "0x")
	hex.Encode(enc[2:], b)
	return string(enc)
}

// Encode encodes b as a hex string without 0x prefix.
func EncodeNo0x(b []byte) string {
	enc := make([]byte, len(b)*2)
	hex.Encode(enc, b)
	return string(enc)
}

func FromHex(s string) []byte {
	s = strip0x(s)
	if len(s)%2 == 1 {
		s = "0" + s
	}
	return Hex2Bytes(s)
}

// Has0xPrefix validates str begins with '0x' or '0X'.
func Has0xPrefix(str string) bool {
	return len(str) >= 2 && str[0] == '0' && (str[1] == 'x' || str[1] == 'X')
}

// Hex2Bytes returns the bytes represented by the hexadecimal string str.
func Hex2Bytes(str string) []byte {
	h, _ := hex.DecodeString(str)
	return h
}

func ToHexNoLeadingZeroes(b []byte) string {
	var g big.Int
	g.SetBytes(b)
	return "0x" + g.Text(16)
}

func ToBytesSlice(data []Bytes) [][]byte {
	result := make([][]byte, len(data))
	for i, v := range data {
		result[i] = []byte(v)
	}
	return result
}

func FromBytesSlice(data [][]byte) []Bytes {
	result := make([]Bytes, len(data))
	for i, v := range data {
		result[i] = Bytes(v)
	}
	return result
}
