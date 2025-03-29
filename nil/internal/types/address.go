package types

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math"
	"math/big"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/common/hexutil"
)

// Addr is the expected length of the address (in bytes)
const AddrSize = 20

// ShardIdSize is a size of Address's shard id in bytes
const ShardIdSize = 2

// Address represents the 20-byte address of an Ethereum account.
type Address [AddrSize]byte

var (
	EmptyAddress            = Address{}
	MainSmartAccountAddress = ShardAndHexToAddress(BaseShardId, "111111111111111111111111111111111111")
	FaucetAddress           = ShardAndHexToAddress(BaseShardId, "111111111111111111111111111111111110")
	EthFaucetAddress        = ShardAndHexToAddress(BaseShardId, "111111111111111111111111111111111112")
	UsdtFaucetAddress       = ShardAndHexToAddress(BaseShardId, "111111111111111111111111111111111113")
	BtcFaucetAddress        = ShardAndHexToAddress(BaseShardId, "111111111111111111111111111111111114")
	UsdcFaucetAddress       = ShardAndHexToAddress(BaseShardId, "111111111111111111111111111111111115")
	L1BlockInfoAddress      = ShardAndHexToAddress(MainShardId, "222222222222222222222222222222222222")
	GovernanceAddress       = ShardAndHexToAddress(MainShardId, "777777777777777777777777777777777777")
)

func GetTokenName(addr TokenId) string {
	switch Address(addr) {
	case FaucetAddress:
		return "NIL"
	case EthFaucetAddress:
		return "ETH"
	case UsdtFaucetAddress:
		return "USDT"
	case BtcFaucetAddress:
		return "BTC"
	case UsdcFaucetAddress:
		return "USDC"
	}
	return ""
}

var tokens = map[string]Address{
	"ETH":  EthFaucetAddress,
	"USDT": UsdtFaucetAddress,
	"USDC": UsdcFaucetAddress,
	"BTC":  BtcFaucetAddress,
	"NIL":  FaucetAddress,
}

func GetTokens() map[string]Address {
	return tokens
}

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

// ShardAndHexToAddress returns Address with byte values of ShardId + s.
// If s is larger than `AddrSize - ShardIdSize`, it will panic.
func ShardAndHexToAddress(shardId ShardId, s string) Address {
	addr := HexToAddress(s)
	if addr[0] != 0 || addr[1] != 0 {
		panic("incorrect address length")
	}
	setShardId(addr[:], shardId)
	return addr
}

// Bytes gets the string representation of the underlying address.
func (a Address) Bytes() []byte { return a[:] }

// Hash converts an address to a hash by left-padding it with zeros.
func (a Address) Hash() common.Hash { return common.BytesToHash(a[:]) }

// Hex returns an EIP55-compliant hex string representation of the address.
func (a Address) Hex() string {
	return string(a.hex())
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
		_, _ = s.Write(a.hex())
	case 'q':
		q := []byte{'"'}
		_, _ = s.Write(q)
		_, _ = s.Write(a.hex())
		_, _ = s.Write(q)
	case 'x', 'X':
		// %x disables the checksum.
		h := a.hex()
		if !s.Flag('#') {
			h = h[2:]
		}
		if c == 'X' {
			h = bytes.ToUpper(h)
		}
		_, _ = s.Write(h)
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
		b = b[len(b)-AddrSize:]
	}
	copy(a[AddrSize-len(b):], b)
}

// MarshalText returns the hex representation of a.
func (a Address) MarshalText() ([]byte, error) {
	return hexutil.Bytes(a.Bytes()).MarshalText()
}

func (a *Address) UnmarshalText(input []byte) error {
	return hexutil.UnmarshalFixedText("Address", input, a[:])
}

func setShardId(bytes []byte, shardId ShardId) []byte {
	check.PanicIfNotf(ShardIdSize == 2, "please adjust shard id size")
	check.PanicIfNotf(shardId <= math.MaxUint16, "too big shard id")
	copy(bytes, shardId.Bytes())
	return bytes
}

func (a Address) ShardId() ShardId {
	return BytesToShardId(a[:ShardIdSize])
}

func (a *Address) Set(val string) error {
	return a.UnmarshalText([]byte(val))
}

func (a *Address) Type() string {
	return "Address"
}

func createAddress(shardId ShardId, deployPayload []byte) Address {
	raw := make([]byte, ShardIdSize, AddrSize)
	raw = setShardId(raw, shardId)
	offset := common.HashSize - AddrSize + ShardIdSize
	raw = append(raw, common.PoseidonHash(deployPayload).Bytes()[offset:]...)
	return BytesToAddress(raw)
}

// CreateAddress creates address for the given contract code + salt
func CreateAddress(shardId ShardId, deployPayload DeployPayload) Address {
	return createAddress(shardId, deployPayload.Bytes())
}

// CreateAddressForCreate2 creates address in a CREATE2-like way
func CreateAddressForCreate2(sender Address, code []byte, salt common.Hash) Address {
	data := make([]byte, 0, 1+AddrSize+2*common.HashSize)
	data = append(data, 0xff)
	data = append(data, sender.Bytes()...)
	data = append(data, salt.Bytes()...)
	data = append(data, common.PoseidonHash(code).Bytes()...)
	return createAddress(sender.ShardId(), data)
}

func GenerateRandomAddress(shardId ShardId) Address {
	b := make([]byte, AddrSize-ShardIdSize)
	_, err := rand.Read(b)
	if err != nil {
		panic(err)
	}

	raw := make([]byte, ShardIdSize, AddrSize)
	raw = setShardId(raw, shardId)
	raw = append(raw, b...)
	return BytesToAddress(raw)
}

func GenerateRandomHash() common.Hash {
	randomBytes := make([]byte, 32)
	_, err := rand.Read(randomBytes)
	if err != nil {
		panic(err)
	}
	return common.BytesToHash(randomBytes)
}

func ToShardedHash(h common.Hash, shardId ShardId) common.Hash {
	raw := h
	setShardId(raw[:], shardId)
	return raw
}

func ShardIdFromHash(h common.Hash) ShardId {
	return BytesToShardId(h[:ShardIdSize])
}
