package transport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/NilFoundation/nil/nil/internal/types"
)

// API describes the set of methods offered over the RPC interface
type API struct {
	Namespace string      // namespace under which the rpc methods of Service are exposed
	Version   string      // api version for DApp's
	Service   interface{} // receiver instance which holds the methods
	Public    bool        // indication if the methods must be considered safe for public use
}

// Error wraps RPC errors, which contain an error code in addition to the message.
type Error interface {
	Error() string  // returns the message
	ErrorCode() int // returns the code
}

// A DataError contains some data in addition to the error message.
type DataError interface {
	Error() string          // returns the message
	ErrorData() interface{} // returns the error data
}

// ServerCodec implements reading, parsing and writing RPC messages for the server side of
// a RPC session. Implementations must be go-routine safe since the codec can be called in
// multiple go-routines concurrently.
type ServerCodec interface {
	Read() (msgs []*Message, isBatch bool, err error)
	Close()
	JsonWriter
}

// JsonWriter can write JSON messages to its underlying connection.
// Implementations must be safe for concurrent use.
type JsonWriter interface {
	WriteJSON(context.Context, interface{}) error
	// Closed returns a channel which is closed when the connection is closed.
	Closed() <-chan interface{}
	// RemoteAddr returns the peer address of the connection.
	RemoteAddr() string
}

type (
	BlockNumber int64
	Timestamp   uint64
)

const (
	LatestExecutedBlockNumber = BlockNumber(-5)
	FinalizedBlockNumber      = BlockNumber(-4)
	SafeBlockNumber           = BlockNumber(-3)
	PendingBlockNumber        = BlockNumber(-2)
	LatestBlockNumber         = BlockNumber(-1)
	EarliestBlockNumber       = BlockNumber(0)

	Earliest       = "earliest"
	Latest         = "latest"
	Pending        = "pending"
	Safe           = "safe"
	Finalized      = "finalized"
	LatestExecuted = "latestExecuted"
)

var (
	LatestExecutedBlock = LatestExecutedBlockNumber.AsBlockReference()
	FinalizedBlock      = FinalizedBlockNumber.AsBlockReference()
	SafeBlock           = SafeBlockNumber.AsBlockReference()
	PendingBlock        = PendingBlockNumber.AsBlockReference()
	LatestBlock         = LatestBlockNumber.AsBlockReference()
	EarliestBlock       = EarliestBlockNumber.AsBlockReference()
)

// UnmarshalText parses the given string into a BlockNumber. It supports:
// - "latest", "earliest", "pending", "safe", or "finalized" as string arguments
// - the block number
// Returned errors:
// - an invalid block number error when the given argument isn't a known strings
// - an out of range error when the given block number is either too little or too large
func (bn *BlockNumber) UnmarshalText(data []byte) error {
	input := strings.TrimSpace(string(data))
	if len(input) >= 2 && input[0] == '"' && input[len(input)-1] == '"' {
		input = input[1 : len(input)-1]
	}

	switch input {
	case Earliest:
		*bn = EarliestBlockNumber
		return nil
	case Latest:
		*bn = LatestBlockNumber
		return nil
	case Pending:
		*bn = PendingBlockNumber
		return nil
	case Safe:
		*bn = SafeBlockNumber
		return nil
	case Finalized:
		*bn = FinalizedBlockNumber
		return nil
	case LatestExecuted:
		*bn = LatestExecutedBlockNumber
		return nil
	case "null":
		*bn = LatestBlockNumber
		return nil
	}

	// Try to parse it as a number
	blckNum, err := strconv.ParseUint(input, 10, 64)
	if err != nil {
		// Now try as a hex number
		if blckNum, err = hexutil.DecodeUint64(input); err != nil {
			return err
		}
	}
	if blckNum > math.MaxInt64 {
		return errors.New("block number larger than int64")
	}
	*bn = BlockNumber(blckNum)
	return nil
}

func (bn BlockNumber) MarshalText() ([]byte, error) {
	switch {
	case bn < LatestExecutedBlockNumber:
		return nil, fmt.Errorf("invalid block number %d", bn)
	case bn < 0:
		return []byte(bn.String()), nil
	default:
		return []byte(bn.string(16)), nil
	}
}

func (bn BlockNumber) Int64() int64 {
	return int64(bn)
}

func (bn BlockNumber) Uint64() uint64 {
	return uint64(bn)
}

func (bn BlockNumber) IsSpecial() bool {
	return bn < 0
}

func (bn BlockNumber) BlockNumber() types.BlockNumber {
	if bn < 0 {
		panic(fmt.Sprintf("A special value of BlockNumber is used as a real value: %d", bn))
	}
	return types.BlockNumber(bn.Uint64())
}

func (bn BlockNumber) String() string {
	return bn.string(10)
}

func (bn *BlockNumber) Set(s string) error {
	return bn.UnmarshalText([]byte(s))
}

func (bn BlockNumber) Type() string {
	return "BlockNumber"
}

func (bn BlockNumber) AsBlockReference() BlockReference {
	res, _ := AsBlockReference(bn)
	return res
}

func (bn BlockNumber) string(base int) string {
	switch bn {
	case EarliestBlockNumber:
		return Earliest
	case LatestBlockNumber:
		return Latest
	case PendingBlockNumber:
		return Pending
	case SafeBlockNumber:
		return Safe
	case FinalizedBlockNumber:
		return Finalized
	case LatestExecutedBlockNumber:
		return LatestExecuted
	}

	if base == 16 {
		return "0x" + strconv.FormatUint(bn.Uint64(), base)
	}

	return strconv.FormatUint(bn.Uint64(), base)
}

type BlockNumberOrHash struct {
	BlockNumber      *BlockNumber `json:"blockNumber,omitempty"`
	BlockHash        *common.Hash `json:"blockHash,omitempty"`
	RequireCanonical bool         `json:"requireCanonical,omitempty"`
}

func (bnh *BlockNumberOrHash) UnmarshalJSON(data []byte) error {
	type erased BlockNumberOrHash
	e := erased{}
	err := json.Unmarshal(data, &e)
	if err == nil {
		if e.BlockNumber != nil && e.BlockHash != nil {
			return errors.New("cannot specify both BlockHash and BlockNumber, choose one or the other")
		}
		if e.BlockNumber == nil && e.BlockHash == nil {
			return errors.New("at least one of BlockNumber or BlockHash is needed if a dictionary is provided")
		}
		bnh.BlockNumber = e.BlockNumber
		bnh.BlockHash = e.BlockHash
		bnh.RequireCanonical = e.RequireCanonical
		return nil
	}
	// Try simple number first
	blckNum, err := strconv.ParseUint(string(data), 10, 64)
	if err == nil {
		if blckNum > math.MaxInt64 {
			return errors.New("block number too high")
		}
		bn := BlockNumber(blckNum)
		bnh.BlockNumber = &bn
		return nil
	}

	input := strings.TrimSpace(string(data))
	if len(input) >= 2 && input[0] == '"' && input[len(input)-1] == '"' {
		input = input[1 : len(input)-1]
	}
	switch input {
	case Earliest:
		bn := EarliestBlockNumber
		bnh.BlockNumber = &bn
		return nil
	case Latest:
		bn := LatestBlockNumber
		bnh.BlockNumber = &bn
		return nil
	case Pending:
		bn := PendingBlockNumber
		bnh.BlockNumber = &bn
		return nil
	case Safe:
		bn := SafeBlockNumber
		bnh.BlockNumber = &bn
		return nil
	case Finalized:
		bn := FinalizedBlockNumber
		bnh.BlockNumber = &bn
		return nil
	default:
		if len(input) == 66 {
			hash := common.EmptyHash
			if err = hash.UnmarshalText([]byte(input)); err != nil {
				return err
			}
			bnh.BlockHash = &hash
			return nil
		} else {
			if blckNum, err = hexutil.DecodeUint64(input); err != nil {
				return err
			}
			if blckNum > math.MaxInt64 {
				return errors.New("block number too high")
			}
			bn := BlockNumber(blckNum)
			bnh.BlockNumber = &bn
			return nil
		}
	}
}

func (bnh *BlockNumberOrHash) Number() (BlockNumber, bool) {
	if bnh.BlockNumber != nil {
		return *bnh.BlockNumber, true
	}
	return BlockNumber(0), false
}

func (bnh *BlockNumberOrHash) Hash() (common.Hash, bool) {
	if bnh.BlockHash != nil {
		return *bnh.BlockHash, true
	}
	return common.EmptyHash, false
}

type BlockReference BlockNumberOrHash

func (br *BlockReference) UnmarshalJSON(data []byte) error {
	return ((*BlockNumberOrHash)(br)).UnmarshalJSON(data)
}

func (br BlockReference) Number() (BlockNumber, bool) {
	return ((*BlockNumberOrHash)(&br)).Number()
}

func (br BlockReference) Hash() (common.Hash, bool) {
	return ((*BlockNumberOrHash)(&br)).Hash()
}

func (br BlockReference) String() string {
	if br.BlockNumber != nil {
		return br.BlockNumber.String()
	}

	if br.BlockHash != nil {
		return br.BlockHash.String()
	}

	return ""
}

func (br BlockReference) IsValid() bool {
	return br.BlockHash != nil || br.BlockNumber != nil
}

func (br BlockReference) Type() string {
	return "BlockReference"
}

func (br *BlockReference) Set(v string) (err error) {
	value, err := AsBlockReference(v)
	if err != nil {
		return err
	}
	*br = value
	return
}

func AsBlockReference(ref any) (BlockReference, error) {
	switch ref := ref.(type) {
	case BlockNumberOrHash:
		return BlockReference(ref), nil
	case *BlockNumberOrHash:
		return BlockReference(*ref), nil
	case BlockReference:
		return ref, nil
	case *BlockReference:
		return *ref, nil
	case *big.Int:
		return IntBlockReference(ref), nil
	case BlockNumber:
		return BlockReference{BlockNumber: &ref}, nil
	case *BlockNumber:
		return BlockReference{BlockNumber: ref}, nil
	case int:
		bn := BlockNumber(ref)
		return BlockReference{BlockNumber: &bn}, nil
	case int64:
		bn := BlockNumber(ref)
		return BlockReference{BlockNumber: &bn}, nil
	case uint64:
		return Uint64BlockReference(ref), nil
	case common.Hash:
		return HashBlockReference(ref), nil
	case *common.Hash:
		return HashBlockReference(*ref), nil
	case string:
		br := &BlockReference{}
		if err := br.UnmarshalJSON([]byte(ref)); err != nil {
			return BlockReference{}, err
		}
		return *br, nil
	}

	return BlockReference{}, nil
}

func IntBlockReference(blockNr *big.Int) BlockReference {
	if blockNr == nil {
		return BlockReference{}
	}

	bn := BlockNumber(blockNr.Int64())
	return BlockReference{
		BlockNumber:      &bn,
		BlockHash:        nil,
		RequireCanonical: false,
	}
}

func Uint64BlockReference(blockNr uint64) BlockReference {
	bn := BlockNumber(blockNr)
	return BlockReference{
		BlockNumber:      &bn,
		BlockHash:        nil,
		RequireCanonical: false,
	}
}

func HashBlockReference(hash common.Hash, canonical ...bool) BlockReference {
	if len(canonical) == 0 {
		canonical = []bool{false}
	}

	return BlockReference{
		BlockNumber:      nil,
		BlockHash:        &hash,
		RequireCanonical: canonical[0],
	}
}

func (ts *Timestamp) UnmarshalJSON(data []byte) error {
	input := strings.TrimSpace(string(data))
	if len(input) >= 2 && input[0] == '"' && input[len(input)-1] == '"' {
		input = input[1 : len(input)-1]
	}

	// parse string to uint64
	timestamp, err := strconv.ParseUint(input, 10, 64)
	if err != nil {
		// try hex number
		if timestamp, err = hexutil.DecodeUint64(input); err != nil {
			return err
		}
	}

	*ts = Timestamp(timestamp)
	return nil
}
