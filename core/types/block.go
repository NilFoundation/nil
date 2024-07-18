package types

import (
	"math"
	"strconv"

	fastssz "github.com/NilFoundation/fastssz"
	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/check"
	"github.com/NilFoundation/nil/common/hexutil"
	"github.com/NilFoundation/nil/common/ssz"
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
	MainChainHash       common.Hash  `ch:"main_chain_hash"`
	LogsBloom           Bloom        `ch:"logs_bloom"`
	Timestamp           uint64       `ch:"timestamp"`
	GasPrice            Value        `ch:"gas_price"`
}

type BlockWithRawExtractedData struct {
	*Block
	InMessages  []ssz.SSZEncodedData
	OutMessages []ssz.SSZEncodedData
	Receipts    []ssz.SSZEncodedData
	Errors      map[common.Hash]string
}

type BlockWithExtractedData struct {
	*Block
	InMessages  []*Message
	OutMessages []*Message
	Receipts    []*Receipt
	Errors      map[common.Hash]string
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

func (b *Block) ToHexedSSZ() (string, error) {
	content, err := b.MarshalSSZ()
	if err != nil {
		return "", err
	}
	return hexutil.Encode(content), nil
}

func (b *Block) FromHexedSSZ(hexData string) error {
	return b.UnmarshalSSZ(hexutil.FromHex(hexData))
}

func BlockFromHexedSSZ(hexData string) (*Block, error) {
	block := new(Block)
	if err := block.FromHexedSSZ(hexData); err != nil {
		return nil, err
	}
	return block, nil
}

func (b *BlockWithRawExtractedData) DecodeSSZ() (*BlockWithExtractedData, error) {
	inMessages, err := ssz.DecodeContainer[Message, *Message](b.InMessages)
	if err != nil {
		return nil, err
	}
	outMessages, err := ssz.DecodeContainer[Message, *Message](b.OutMessages)
	if err != nil {
		return nil, err
	}
	receipts, err := ssz.DecodeContainer[Receipt, *Receipt](b.Receipts)
	if err != nil {
		return nil, err
	}
	return &BlockWithExtractedData{
		Block:       b.Block,
		InMessages:  inMessages,
		OutMessages: outMessages,
		Receipts:    receipts,
		Errors:      b.Errors,
	}, nil
}

func (b *BlockWithExtractedData) EncodeSSZ() (*BlockWithRawExtractedData, error) {
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
	return &BlockWithRawExtractedData{
		Block:       b.Block,
		InMessages:  inMessages,
		OutMessages: outMessages,
		Receipts:    receipts,
		Errors:      b.Errors,
	}, nil
}

const InvalidDbTimestamp uint64 = math.MaxUint64
