package types

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math"
	"math/big"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/hexutil"
	ssz "github.com/ferranbt/fastssz"
	"github.com/iden3/go-iden3-crypto/poseidon"
)

// Address represents the 20 byte address of an Ethereum account.
type Address [common.AddrSize]byte

var EmptyAddress = Address{}

// BytesToAddress returns Address with value b.
// If b is larger than len(h), b will be cropped from the left.
func BytesToAddress(b []byte) Address {
	var a Address
	a.SetBytes(b)
	return a
}

// BigToAddress returns Address with byte values of b.
// If b is larger than len(h), b will be cropped from the left.
func BigToAddress(b *big.Int) Address { return BytesToAddress(b.Bytes()) }

// HexToAddress returns Address with byte values of s.
// If s is larger than len(h), s will be cropped from the left.
func HexToAddress(s string) Address {
	if hexutil.Has0xPrefix(s) {
		s = s[2:]
	}
	b, err := hex.DecodeString(s)
	if err != nil {
		return Address{}
	}

	return BytesToAddress(b)
}

// IsHexAddress verifies whether a string can represent a valid hex-encoded
// Ethereum address or not.
func IsHexAddress(s string) bool {
	if len(s) >= 2 && s[0] == '0' && (s[1] == 'x' || s[1] == 'X') {
		s = s[2:]
	}
	_, err := hex.DecodeString(s)
	return err == nil
}

// Bytes gets the string representation of the underlying address.
func (a Address) Bytes() []byte { return a[:] }

// Hash converts an address to a hash by left-padding it with zeros.
func (a Address) Hash() common.Hash { return common.BytesToHash(a[:]) }

// Hex returns an EIP55-compliant hex string representation of the address.
func (a Address) Hex() string {
	return string(a.checksumHex())
}

func (a Address) Equal(b Address) bool {
	return bytes.Equal(a.Bytes(), b.Bytes())
}

func (a Address) IsEmpty() bool {
	return a.Equal(EmptyAddress)
}

// String implements fmt.Stringer.
func (a Address) String() string {
	return a.Hex()
}

func (a *Address) checksumHex() []byte {
	buf := a.hex()

	// compute checksum
	hash := poseidon.Sum(buf[2:])

	for i := 2; i < len(buf); i++ {
		hashByte := hash[(i-2)/2]
		if i%2 == 0 {
			hashByte >>= 4
		} else {
			hashByte &= 0xf
		}
		if buf[i] > '9' && hashByte > 7 {
			buf[i] -= 32
		}
	}
	return buf
}

func (a Address) hex() []byte {
	var buf [len(a)*2 + 2]byte
	copy(buf[:2], "0x")
	hex.Encode(buf[2:], a[:])
	return buf[:]
}

// Format implements fmt.Formatter.
// Address supports the %v, %s, %v, %x, %X and %d format verbs.
func (a Address) Format(s fmt.State, c rune) {
	switch c {
	case 'v', 's':
		s.Write(a.checksumHex())
	case 'q':
		q := []byte{'"'}
		s.Write(q)
		s.Write(a.checksumHex())
		s.Write(q)
	case 'x', 'X':
		// %x disables the checksum.
		hex := a.hex()
		if !s.Flag('#') {
			hex = hex[2:]
		}
		if c == 'X' {
			hex = bytes.ToUpper(hex)
		}
		s.Write(hex)
	case 'd':
		fmt.Fprint(s, ([len(a)]byte)(a))
	default:
		fmt.Fprintf(s, "%%!%c(address=%x)", c, a)
	}
}

// SetBytes sets the address to the value of b.
// If b is larger than len(a), b will be cropped from the left.
func (a *Address) SetBytes(b []byte) {
	if len(b) > len(a) {
		b = b[len(b)-common.AddrSize:]
	}
	copy(a[common.AddrSize-len(b):], b)
}

// MarshalText returns the hex representation of a.
func (a Address) MarshalText() ([]byte, error) {
	return hexutil.Bytes(a.Bytes()).MarshalText()
}

func (a *Address) UnmarshalText(input []byte) error {
	return hexutil.UnmarshalFixedText("Address", input, a[:])
}

func appendShardId(bytes []byte, shardId uint32) []byte {
	if shardId > math.MaxUint16 {
		panic("invalid shardId value")
	}
	binary.BigEndian.PutUint16(bytes, uint16(shardId))
	return bytes
}

func (a Address) ShardId() uint32 {
	num := binary.BigEndian.Uint16(a[:2])
	return uint32(num)
}

func PubkeyBytesToAddress(shardId uint32, pubBytes []byte) Address {
	bytes := make([]byte, 2, common.AddrSize)
	bytes = appendShardId(bytes, shardId)
	bytes = append(bytes, common.PoseidonHash(pubBytes).Bytes()[14:]...)
	return BytesToAddress(bytes)
}

// CreateAddress creates an address given the bytes and the nonce.
func CreateAddress(shardId uint32, b Address, nonce uint64) Address {
	bytes := make([]byte, 2, common.AddrSize)
	bytes = appendShardId(bytes, shardId)

	buf := make([]byte, len(b)+8)
	copy(buf, b.Bytes())
	buf = ssz.MarshalUint64(buf, nonce)

	bytes = append(bytes, common.PoseidonHash(buf).Bytes()[14:]...)
	return BytesToAddress(bytes)
}

func GenerateRandomAddress(shardId uint32) Address {
	b := make([]byte, 18)
	_, err := rand.Read(b)
	if err != nil {
		panic(err)
	}

	bytes := make([]byte, 2, common.AddrSize)
	bytes = appendShardId(bytes, shardId)
	bytes = append(bytes, b...)
	return BytesToAddress(bytes)
}
