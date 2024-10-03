package types

import (
	"math"
	"strconv"

	fastssz "github.com/NilFoundation/fastssz"
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/ssz"
)

type BlockNumber uint64

func (bn BlockNumber) Uint64() uint64 {
	return uint64(bn)
}

func (bn BlockNumber) String() string { return strconv.FormatUint(bn.Uint64(), 10) }
func (bn BlockNumber) Bytes() []byte  { return []byte(bn.String()) }
func (bn BlockNumber) Type() string   { return "BlockNumber" }

func (bn *BlockNumber) Set(s string) error {
	v, err := strconv.ParseUint(s, 0, 64)
	if err != nil {
		return err
	}
	*bn = BlockNumber(v)
	return nil
}

type Block struct {
	Id                 BlockNumber `json:"id" ch:"id"`
	PrevBlock          common.Hash `json:"prevBlock" ch:"prev_block"`
	SmartContractsRoot common.Hash `json:"smartContractsRoot" ch:"smart_contracts_root"`
	InMessagesRoot     common.Hash `json:"inMessagesRoot" ch:"in_messages_root"`
	// OutMessagesRoot stores all outbound messages produced by transactions of this block. The key of the tree is a
	// sequential index of the message, value is a Message struct.
	// It can be considered as an array, where each segment is referred by corresponding receipt.
	OutMessagesRoot common.Hash `json:"outMessagesRoot" ch:"out_messages_root"`
	// We cache the size of out messages, otherwise we should iterate all the tree to get its size
	OutMessagesNum      MessageIndex `json:"outMessagesNum" ch:"out_messages_num"`
	ReceiptsRoot        common.Hash  `json:"receiptsRoot" ch:"receipts_root"`
	ChildBlocksRootHash common.Hash  `json:"childBlocksRootHash" ch:"child_blocks_root_hash"`
	MainChainHash       common.Hash  `json:"mainChainHash" ch:"main_chain_hash"`
	ConfigRoot          common.Hash  `json:"configRoot" ch:"config_root"`
	LogsBloom           Bloom        `json:"logsBloom" ch:"logs_bloom"`
	Timestamp           uint64       `json:"timestamp" ch:"timestamp"`
	GasPrice            Value        `json:"gasPrice" ch:"gas_price"`
}

type RawBlockWithExtractedData struct {
	Block       ssz.SSZEncodedData
	InMessages  []ssz.SSZEncodedData
	OutMessages []ssz.SSZEncodedData
	Receipts    []ssz.SSZEncodedData
	Errors      map[common.Hash]string
	ChildBlocks []common.Hash
	DbTimestamp uint64
}

type BlockWithExtractedData struct {
	*Block
	InMessages  []*Message             `json:"inMessages"`
	OutMessages []*Message             `json:"outMessages"`
	Receipts    []*Receipt             `json:"receipts"`
	Errors      map[common.Hash]string `json:"errors,omitempty"`
	ChildBlocks []common.Hash          `json:"childBlocks"`
	DbTimestamp uint64                 `json:"dbTimestamp"`
}

// interfaces
var (
	_ common.Hashable     = new(Block)
	_ fastssz.Marshaler   = new(Block)
	_ fastssz.Unmarshaler = new(Block)
)

func (b *Block) Hash() common.Hash {
	return common.MustPoseidonSSZ(b)
}

func (b *RawBlockWithExtractedData) DecodeSSZ() (*BlockWithExtractedData, error) {
	block := &Block{}
	if err := block.UnmarshalSSZ(b.Block); err != nil {
		return nil, err
	}
	inMessages, err := ssz.DecodeContainer[*Message](b.InMessages)
	if err != nil {
		return nil, err
	}
	outMessages, err := ssz.DecodeContainer[*Message](b.OutMessages)
	if err != nil {
		return nil, err
	}
	receipts, err := ssz.DecodeContainer[*Receipt](b.Receipts)
	if err != nil {
		return nil, err
	}
	return &BlockWithExtractedData{
		Block:       block,
		InMessages:  inMessages,
		OutMessages: outMessages,
		Receipts:    receipts,
		Errors:      b.Errors,
		ChildBlocks: b.ChildBlocks,
		DbTimestamp: b.DbTimestamp,
	}, nil
}

func (b *BlockWithExtractedData) EncodeSSZ() (*RawBlockWithExtractedData, error) {
	block, err := b.Block.MarshalSSZ()
	if err != nil {
		return nil, err
	}
	inMessages, err := ssz.EncodeContainer(b.InMessages)
	if err != nil {
		return nil, err
	}
	outMessages, err := ssz.EncodeContainer(b.OutMessages)
	if err != nil {
		return nil, err
	}
	receipts, err := ssz.EncodeContainer(b.Receipts)
	if err != nil {
		return nil, err
	}
	return &RawBlockWithExtractedData{
		Block:       block,
		InMessages:  inMessages,
		OutMessages: outMessages,
		Receipts:    receipts,
		Errors:      b.Errors,
		ChildBlocks: b.ChildBlocks,
		DbTimestamp: b.DbTimestamp,
	}, nil
}

const InvalidDbTimestamp uint64 = math.MaxUint64
