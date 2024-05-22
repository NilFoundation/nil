package types

import (
	"strconv"

	"github.com/NilFoundation/nil/common"
	fastssz "github.com/ferranbt/fastssz"
)

type BlockNumber uint64

func (bn BlockNumber) Uint64() uint64 {
	return uint64(bn)
}

func (bn BlockNumber) String() string { return strconv.FormatUint(bn.Uint64(), 10) }
func (bn BlockNumber) Bytes() []byte  { return []byte(bn.String()) }

type Block struct {
	Id                 BlockNumber
	PrevBlock          common.Hash
	SmartContractsRoot common.Hash
	InMessagesRoot     common.Hash
	// OutMessagesRoot stores all outbound messages produced by transactions of this block. The key of the tree is a
	// sequential index of the message, value is a Message struct.
	// It can be considered as an array, where each segment is referred by corresponding receipt.
	OutMessagesRoot common.Hash
	// We cache size of out messages, otherwise we should iterate all the tree to get its size
	OutMessagesNum      uint32
	ReceiptsRoot        common.Hash
	ChildBlocksRootHash common.Hash
	MasterChainHash     common.Hash
	LogsBloom           Bloom
	Timestamp           uint64
}

// interfaces
var (
	_ common.Hashable     = new(Block)
	_ fastssz.Marshaler   = new(Block)
	_ fastssz.Unmarshaler = new(Block)
)

func (b *Block) Hash() common.Hash {
	h, err := common.PoseidonSSZ(b)
	common.FatalIf(err, nil, "Can't get block hash")

	return h
}
