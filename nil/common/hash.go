package common

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"math/big"
	"math/rand"
	"reflect"

	ssz "github.com/NilFoundation/fastssz"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/holiman/uint256"
	"github.com/iden3/go-iden3-crypto/poseidon"
)

type Hash [HashSize]byte

type Hashable interface {
	Hash() Hash
}

var EmptyHash = Hash{}

// BytesToHash sets b to hash.
// If b is larger than len(h), b will be cropped from the left.
func BytesToHash(b []byte) Hash {
	var h Hash
	h.SetBytes(b)
	return h
}

// PoseidonHash returns 32-bytes poseidon hash of b bytes.
func PoseidonHash(b []byte) Hash {
	if len(b) == 0 {
		return EmptyHash
	}
	return BytesToHash(poseidon.Sum(b))
}

func PoseidonSSZ(data ssz.Marshaler) (Hash, error) {
	buf, err := data.MarshalSSZ()
	if err != nil {
		return EmptyHash, err
	}
	return PoseidonHash(buf), nil
}

func MustPoseidonSSZ(data ssz.Marshaler) Hash {
	h, err := PoseidonSSZ(data)
	check.PanicIfErr(err)
	return h
}

// CastToHash - sets b to hash
// If b is larger than len(h), b will be cropped from the left.
// panics if input is shorter than 32 bytes, see https://go.dev/doc/go1.17#language
// faster than BytesToHash
func CastToHash(b []byte) Hash { return *(*Hash)(b) }

// BigToHash sets byte representation of b to hash.
// If b is larger than len(h), b will be cropped from the left.
func BigToHash(b *big.Int) Hash { return BytesToHash(b.Bytes()) }

func IntToHash(i int) Hash {
	b := big.NewInt(int64(i))
	return BigToHash(b)
}

func (h Hash) Empty() bool {
	return h == EmptyHash
}

// Bytes gets the byte representation of the underlying hash.
func (h Hash) Bytes() []byte { return h[:] }

// Big converts a hash to a big integer.
func (h Hash) Big() *big.Int { return new(big.Int).SetBytes(h[:]) }

func (h Hash) Uint256() *uint256.Int {
	u := uint256.NewInt(0)
	u.SetBytes(h.Bytes())
	return u
}

// TerminalString implements log.TerminalStringer, formatting a string for console
// output during logging.
func (h Hash) TerminalString() string {
	return fmt.Sprintf("%x…%x", h[:3], h[29:])
}

func (h Hash) Hex() string {
	enc := make([]byte, len(h[:])*2+2)
	copy(enc, "0x")
	hex.Encode(enc[2:], h[:])
	return string(enc)
}

// String implements the stringer interface and is used also by the logger when
// doing full logging into a file.
func (h Hash) String() string {
	return h.Hex()
}

// Format implements fmt.Formatter.
// Hash supports the %v, %s, %v, %x, %X and %d format verbs.
func (h Hash) Format(s fmt.State, c rune) {
	hexb := make([]byte, 2+len(h)*2)
	copy(hexb, "0x")
	hex.Encode(hexb[2:], h[:])

	switch c {
	case 'x', 'X':
		if !s.Flag('#') {
			hexb = hexb[2:]
		}
		if c == 'X' {
			hexb = bytes.ToUpper(hexb)
		}
		fallthrough
	case 'v', 's':
		_, _ = s.Write(hexb)
	case 'q':
		q := []byte{'"'}
		_, _ = s.Write(q)
		_, _ = s.Write(hexb)
		_, _ = s.Write(q)
	case 'd':
		fmt.Fprint(s, ([len(h)]byte)(h))
	default:
		fmt.Fprintf(s, "%%!%c(hash=%x)", c, h)
	}
}

func (h *Hash) UnmarshalText(input []byte) error {
	return hexutil.UnmarshalFixedText("Hash", input, h[:])
}

// MarshalText returns the hex representation of h.
func (h Hash) MarshalText() ([]byte, error) {
	return hexutil.Bytes(h.Bytes()).MarshalText()
}

func (h *Hash) Set(val string) error {
	return h.UnmarshalText([]byte(val))
}

func (h *Hash) Type() string {
	return "Hash"
}

// SetBytes sets the hash to the value of b.
// If b is larger than len(h), b will be cropped from the left.
func (h *Hash) SetBytes(b []byte) {
	if len(b) > len(h) {
		b = b[len(b)-HashSize:]
	}

	copy(h[HashSize-len(b):], b)
}

// Generate implements testing/quick.Generator.
func (h Hash) Generate(rand *rand.Rand, size int) reflect.Value {
	m := rand.Intn(len(h))
	for i := len(h) - 1; i > m; i-- {
		h[i] = byte(rand.Uint32())
	}
	return reflect.ValueOf(h)
}

// HexToHash sets byte representation of s to hash.
// If b is larger than len(h), b will be cropped from the left.
func HexToHash(s string) Hash { return BytesToHash(hexutil.FromHex(s)) }

func (h *Hash) UnmarshalSSZ(buf []byte) error {
	*h = BytesToHash(buf)
	return nil
}

func (h *Hash) MarshalSSZ() ([]byte, error) {
	return ssz.MarshalSSZ(h)
}

func (h *Hash) MarshalSSZTo(dest []byte) ([]byte, error) {
	return append(dest, h.Bytes()...), nil
}

func (h *Hash) SizeSSZ() int {
	return HashSize
}
