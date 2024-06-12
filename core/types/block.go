package types

import (
	"strconv"

	fastssz "github.com/NilFoundation/fastssz"
	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/check"
)

type BlockNumber uint64

func (bn BlockNumber) Uint64() uint64 {
	return uint64(bn)
}

func (bn BlockNumber) String() string { return strconv.FormatUint(bn.Uint64(), 10) }
func (bn BlockNumber) Bytes() []byte  { return []byte(bn.String()) }

type Block struct {
	Id                 BlockNumber `ch:"id"`
	PrevBlock          common.Hash `ch:"prev_block"`
	SmartContractsRoot common.Hash `ch:"smart_contracts_root"`
	InMessagesRoot     common.Hash `ch:"in_messages_root"`
	// OutMessagesRoot stores all outbound messages produced by transactions of this block. The key of the tree is a
	// sequential index of the message, value is a Message struct.
	// It can be considered as an array, where each segment is referred by corresponding receipt.
	OutMessagesRoot common.Hash `ch:"out_messages_root"`
	// We cache the size of out messages, otherwise we should iterate all the tree to get its size
	OutMessagesNum      MessageIndex `ch:"out_messages_num"`
	ReceiptsRoot        common.Hash  `ch:"receipts_root"`
	ChildBlocksRootHash common.Hash  `ch:"child_blocks_root_hash"`
	MasterChainHash     common.Hash  `ch:"master_chain_hash"`
	LogsBloom           Bloom        `ch:"logs_bloom"`
	Timestamp           uint64       `ch:"timestamp"`
}

// interfaces
var (
	_ common.Hashable     = new(Block)
	_ fastssz.Marshaler   = new(Block)
	_ fastssz.Unmarshaler = new(Block)
)

func (b *Block) Hash() common.Hash {
	h, err := common.PoseidonSSZ(b)
	check.PanicIfErr(err)
	return h
}
