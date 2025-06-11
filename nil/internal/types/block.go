package types

import (
	"math"
	"strconv"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/crypto/bls"
	"github.com/NilFoundation/nil/nil/internal/serialization"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
)

type BlockNumber uint64

const InvalidBlockNumber BlockNumber = math.MaxUint64

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
	// OutTransactionsRoot stores all outbound transactions produced by transactions of this block.
	// The key of the tree is a sequential index of the transaction, value is a Transaction struct.
	// It can be considered as an array, where each segment is referred by corresponding receipt.
	OutTransactionsRoot common.Hash `json:"outTransactionsRoot" ch:"out_transactions_root"`
	// We cache the size of out transactions, otherwise we should iterate all the tree to get its size
	OutTransactionsNum  TransactionIndex `json:"outTransactionsNum" ch:"out_transaction_num"`
	ReceiptsRoot        common.Hash      `json:"receiptsRoot" ch:"receipts_root"`
	ChildBlocksRootHash common.Hash      `json:"childBlocksRootHash" ch:"child_blocks_root_hash"`
	MainShardHash       common.Hash      `json:"mainShardHash" ch:"main_chain_hash"`
	ConfigRoot          common.Hash      `json:"configRoot" ch:"config_root"`
	BaseFee             Value            `json:"gasPrice" ch:"gas_price"`
	GasUsed             Gas              `json:"gasUsed" ch:"gas_used"`
	L1BlockNumber       uint64           `json:"l1BlockNumber" ch:"l1_block_number"`

	// Incremented after every rollback, used to prevent rollback replay attacks
	RollbackCounter uint32 `json:"rollbackCounter" ch:"rollback_counter"`
	// Required validator patchLevel, incremented if validator updates
	// are required to mitigate an issue
	PatchLevel uint32 `json:"patchLevel" ch:"patch_level"`
}

func (bd BlockData) MarshalNil() ([]byte, error) {
	return rlp.EncodeToBytes(&bd)
}

type ConsensusParams struct {
	ProposerIndex uint64                 `json:"proposerIndex" ch:"round"`
	Round         uint64                 `json:"round" ch:"round"`
	Signature     *BlsAggregateSignature `json:"signature" ch:"-" rlp:"optional"`
}

type Block struct {
	BlockData
	LogsBloom ethtypes.Bloom `json:"logsBloom" ch:"logs_bloom"`
	ConsensusParams
}

type BlockWithHash struct {
	*Block

	Hash common.Hash
}

func NewBlockWithHash(b *Block, shardId ShardId) *BlockWithHash {
	return &BlockWithHash{
		Block: b,
		Hash:  b.Hash(shardId),
	}
}

func NewBlockWithRawHash(b *Block, rawHash common.Hash) *BlockWithHash {
	return &BlockWithHash{
		Block: b,
		Hash:  rawHash,
	}
}

type RawBlockWithExtractedData struct {
	Block           serialization.EncodedData
	InTransactions  []serialization.EncodedData
	InTxCounts      []serialization.EncodedData
	OutTransactions []serialization.EncodedData
	OutTxCounts     []serialization.EncodedData
	Receipts        []serialization.EncodedData
	Errors          map[common.Hash]string
	ChildBlocks     []common.Hash
	DbTimestamp     uint64
	Config          map[string][]byte
}

type TxCount struct {
	ShardId uint16           `json:"shardId"`
	Count   TransactionIndex `json:"count"`
}

func (c *TxCount) UnmarshalNil(buf []byte) error {
	return rlp.DecodeBytes(buf, c)
}

func (c TxCount) MarshalNil() ([]byte, error) {
	return rlp.EncodeToBytes(&c)
}

type BlockWithExtractedData struct {
	*Block
	InTransactions  []*Transaction           `json:"inTransactions"`
	InTxCounts      []*TxCount               `json:"inTxCounts"`
	OutTransactions []*Transaction           `json:"outTransactions"`
	OutTxCounts     []*TxCount               `json:"outTxCounts"`
	Receipts        []*Receipt               `json:"receipts"`
	Errors          map[common.Hash]string   `json:"errors,omitempty"`
	ChildBlocks     []common.Hash            `json:"childBlocks"`
	DbTimestamp     uint64                   `json:"dbTimestamp"`
	Config          map[string]hexutil.Bytes `json:"config"`
}

// interfaces
var (
	_ serialization.NilMarshaler   = new(Block)
	_ serialization.NilUnmarshaler = new(Block)
)

func (b *Block) Hash(shardId ShardId) common.Hash {
	return ToShardedHash(common.MustKeccak(&b.BlockData), shardId)
}

func (b *Block) GetMainShardHash(shardId ShardId) common.Hash {
	if shardId.IsMainShard() {
		return b.PrevBlock
	}
	return b.MainShardHash
}

func (b *Block) UnmarshalNil(buf []byte) error {
	return rlp.DecodeBytes(buf, b)
}

func (b Block) MarshalNil() ([]byte, error) {
	return rlp.EncodeToBytes(&b)
}

func (b *RawBlockWithExtractedData) DecodeBytes() (*BlockWithExtractedData, error) {
	block := &Block{}
	if err := block.UnmarshalNil(b.Block); err != nil {
		return nil, err
	}
	inTransactions, err := serialization.DecodeContainer[*Transaction](b.InTransactions)
	if err != nil {
		return nil, err
	}
	inTxCounts, err := serialization.DecodeContainer[*TxCount](b.InTxCounts)
	if err != nil {
		return nil, err
	}
	outTransactions, err := serialization.DecodeContainer[*Transaction](b.OutTransactions)
	if err != nil {
		return nil, err
	}
	outTxCounts, err := serialization.DecodeContainer[*TxCount](b.OutTxCounts)
	if err != nil {
		return nil, err
	}
	receipts, err := serialization.DecodeContainer[*Receipt](b.Receipts)
	if err != nil {
		return nil, err
	}
	return &BlockWithExtractedData{
		Block:           block,
		InTransactions:  inTransactions,
		InTxCounts:      inTxCounts,
		OutTransactions: outTransactions,
		OutTxCounts:     outTxCounts,
		Receipts:        receipts,
		Errors:          b.Errors,
		ChildBlocks:     b.ChildBlocks,
		DbTimestamp:     b.DbTimestamp,
		Config: common.TransformMap(b.Config, func(k string, v []byte) (string, hexutil.Bytes) {
			return k, v
		}),
	}, nil
}

func (b *BlockWithExtractedData) EncodeToBytes() (*RawBlockWithExtractedData, error) {
	block, err := b.MarshalNil()
	if err != nil {
		return nil, err
	}
	inTransactions, err := serialization.EncodeContainer(b.InTransactions)
	if err != nil {
		return nil, err
	}
	inTxCounts, err := serialization.EncodeContainer(b.InTxCounts)
	if err != nil {
		return nil, err
	}
	outTransactions, err := serialization.EncodeContainer(b.OutTransactions)
	if err != nil {
		return nil, err
	}
	outTxCounts, err := serialization.EncodeContainer(b.OutTxCounts)
	if err != nil {
		return nil, err
	}
	receipts, err := serialization.EncodeContainer(b.Receipts)
	if err != nil {
		return nil, err
	}
	return &RawBlockWithExtractedData{
		Block:           block,
		InTransactions:  inTransactions,
		InTxCounts:      inTxCounts,
		OutTransactions: outTransactions,
		OutTxCounts:     outTxCounts,
		Receipts:        receipts,
		Errors:          b.Errors,
		ChildBlocks:     b.ChildBlocks,
		DbTimestamp:     b.DbTimestamp,
		Config: common.TransformMap(b.Config, func(k string, v hexutil.Bytes) (string, []byte) {
			return k, v
		}),
	}, nil
}

func (b *Block) VerifySignature(pubkeys []bls.PublicKey, shardId ShardId) error {
	sig, err := bls.SignatureFromBytes(b.Signature.Sig)
	if err != nil {
		return err
	}

	mask, err := bls.NewMask(pubkeys)
	if err != nil {
		return err
	}

	if err := mask.SetBytes(b.Signature.Mask); err != nil {
		return err
	}

	aggregatedKey, err := mask.AggregatePublicKeys()
	if err != nil {
		return err
	}

	return sig.Verify(aggregatedKey, b.Hash(shardId).Bytes())
}

const InvalidDbTimestamp uint64 = math.MaxUint64
