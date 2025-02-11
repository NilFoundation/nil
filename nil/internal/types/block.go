package types

import (
	"crypto/ecdsa"
	"errors"
	"math"
	"strconv"

	fastssz "github.com/NilFoundation/fastssz"
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/sszx"
	"github.com/ethereum/go-ethereum/crypto"
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

type BlockData struct {
	Id                 BlockNumber `json:"id" ch:"id"`
	PrevBlock          common.Hash `json:"prevBlock" ch:"prev_block"`
	SmartContractsRoot common.Hash `json:"smartContractsRoot" ch:"smart_contracts_root"`
	InTransactionsRoot common.Hash `json:"inTransactionsRoot" ch:"in_transactions_root"`
	// OutTransactionsRoot stores all outbound transactions produced by transactions of this block. The key of the tree is a
	// sequential index of the transaction, value is a Transaction struct.
	// It can be considered as an array, where each segment is referred by corresponding receipt.
	OutTransactionsRoot common.Hash `json:"outTransactionsRoot" ch:"out_transactions_root"`
	// We cache the size of out transactions, otherwise we should iterate all the tree to get its size
	OutTransactionsNum  TransactionIndex `json:"outTransactionsNum" ch:"out_transaction_num"`
	ReceiptsRoot        common.Hash      `json:"receiptsRoot" ch:"receipts_root"`
	ChildBlocksRootHash common.Hash      `json:"childBlocksRootHash" ch:"child_blocks_root_hash"`
	MainChainHash       common.Hash      `json:"mainChainHash" ch:"main_chain_hash"`
	ConfigRoot          common.Hash      `json:"configRoot" ch:"config_root"`
	Timestamp           uint64           `json:"timestamp" ch:"timestamp"`
	BaseFee             Value            `json:"gasPrice" ch:"gas_price"`
	GasUsed             Gas              `json:"gasUsed" ch:"gas_used"`
	L1BlockNumber       uint64           `json:"l1BlockNumber" ch:"l1_block_number"`
}

type Block struct {
	BlockData
	LogsBloom Bloom     `json:"logsBloom" ch:"logs_bloom"`
	Signature Signature `json:"signature" ch:"signature" ssz-max:"65"`
}

type RawBlockWithExtractedData struct {
	Block           sszx.SSZEncodedData
	InTransactions  []sszx.SSZEncodedData
	OutTransactions []sszx.SSZEncodedData
	Receipts        []sszx.SSZEncodedData
	Errors          map[common.Hash]string
	ChildBlocks     []common.Hash
	DbTimestamp     uint64
}

type BlockWithExtractedData struct {
	*Block
	InTransactions  []*Transaction         `json:"inTransactions"`
	OutTransactions []*Transaction         `json:"outTransactions"`
	Receipts        []*Receipt             `json:"receipts"`
	Errors          map[common.Hash]string `json:"errors,omitempty"`
	ChildBlocks     []common.Hash          `json:"childBlocks"`
	DbTimestamp     uint64                 `json:"dbTimestamp"`
}

// interfaces
var (
	_ fastssz.Marshaler   = new(Block)
	_ fastssz.Unmarshaler = new(Block)
)

func (b *Block) Hash(shardId ShardId) common.Hash {
	return ToShardedHash(common.MustPoseidonSSZ(&b.BlockData), shardId)
}

func (b *RawBlockWithExtractedData) DecodeSSZ() (*BlockWithExtractedData, error) {
	block := &Block{}
	if err := block.UnmarshalSSZ(b.Block); err != nil {
		return nil, err
	}
	inTransactions, err := sszx.DecodeContainer[*Transaction](b.InTransactions)
	if err != nil {
		return nil, err
	}
	outTransactions, err := sszx.DecodeContainer[*Transaction](b.OutTransactions)
	if err != nil {
		return nil, err
	}
	receipts, err := sszx.DecodeContainer[*Receipt](b.Receipts)
	if err != nil {
		return nil, err
	}
	return &BlockWithExtractedData{
		Block:           block,
		InTransactions:  inTransactions,
		OutTransactions: outTransactions,
		Receipts:        receipts,
		Errors:          b.Errors,
		ChildBlocks:     b.ChildBlocks,
		DbTimestamp:     b.DbTimestamp,
	}, nil
}

func (b *BlockWithExtractedData) EncodeSSZ() (*RawBlockWithExtractedData, error) {
	block, err := b.Block.MarshalSSZ()
	if err != nil {
		return nil, err
	}
	inTransactions, err := sszx.EncodeContainer(b.InTransactions)
	if err != nil {
		return nil, err
	}
	outTransactions, err := sszx.EncodeContainer(b.OutTransactions)
	if err != nil {
		return nil, err
	}
	receipts, err := sszx.EncodeContainer(b.Receipts)
	if err != nil {
		return nil, err
	}
	return &RawBlockWithExtractedData{
		Block:           block,
		InTransactions:  inTransactions,
		OutTransactions: outTransactions,
		Receipts:        receipts,
		Errors:          b.Errors,
		ChildBlocks:     b.ChildBlocks,
		DbTimestamp:     b.DbTimestamp,
	}, nil
}

func (b *Block) VerifySignature(pubkey []byte, shardId ShardId) error {
	if len(b.Signature) < 64 || !crypto.VerifySignature(pubkey, b.Hash(shardId).Bytes(), b.Signature[:64]) {
		return errors.New("invalid signature")
	}
	return nil
}

func (b *Block) Sign(prv *ecdsa.PrivateKey, shardId ShardId) error {
	if len(b.Signature) != 0 {
		return errors.New("block is already signed")
	}
	sig, err := crypto.Sign(b.Hash(shardId).Bytes(), prv)
	if err != nil {
		return err
	}
	b.Signature = sig
	return nil
}

const InvalidDbTimestamp uint64 = math.MaxUint64

//go:generate go run github.com/NilFoundation/fastssz/sszgen --path block.go -include ../../common/length.go,signature.go,address.go,code.go,shard.go,bloom.go,log.go,value.go,transaction.go,gas.go,../../common/hash.go --objs BlockData,Block
