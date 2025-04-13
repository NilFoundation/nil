package types

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
)

type Block = jsonrpc.RPCBlock

// BlockRef represents a reference to a specific shard block
type BlockRef struct {
	ShardId types.ShardId     `json:"shardId"`
	Hash    common.Hash       `json:"hash"`
	Number  types.BlockNumber `json:"number"`
}

func NewBlockRef(shardId types.ShardId, hash common.Hash, number types.BlockNumber) BlockRef {
	return BlockRef{
		ShardId: shardId,
		Hash:    hash,
		Number:  number,
	}
}

func BlockToRef(block *Block) BlockRef {
	return NewBlockRef(block.ShardId, block.Hash, block.Number)
}

func (br *BlockRef) String() string {
	return fmt.Sprintf("BlockRef{shardId=%s, number=%d, hash=%s}", br.ShardId, br.Number, br.Hash)
}

// BlockRefs represents per-shard block references
type BlockRefs map[types.ShardId]BlockRef

func (r BlockRefs) TryGet(shard types.ShardId) *BlockRef {
	if ref, ok := r[shard]; ok {
		return &ref
	}
	return nil
}

func (r BlockRefs) TryGetMain() *BlockRef {
	return r.TryGet(types.MainShardId)
}

type BlocksRange struct {
	Start types.BlockNumber
	End   types.BlockNumber
}

// GetBlocksFetchingRange determines the range of blocks to fetch between the latest handled block
// and the actual latest block.
// latestHandled can be equal to:
// a) The latest block fetched from the cluster, or
// b) The latest proved state root, if `latestFetched` is nil.
func GetBlocksFetchingRange(
	latestHandled BlockRef,
	actualLatest BlockRef,
	maxNumBlocks uint32,
) (*BlocksRange, error) {
	if maxNumBlocks == 0 {
		return nil, nil
	}

	var blocksRange BlocksRange
	switch {
	case latestHandled.Number < actualLatest.Number:
		blocksRange = BlocksRange{latestHandled.Number + 1, actualLatest.Number}

	case latestHandled.Number == actualLatest.Number && latestHandled.Hash != actualLatest.Hash:
		return nil, fmt.Errorf(
			"%w: latest blocks have same number %d, but hashes are different: %s != %s",
			ErrBlockMismatch, actualLatest.Number, latestHandled.Hash, actualLatest.Hash,
		)

	case latestHandled.Number > actualLatest.Number:
		return nil, fmt.Errorf(
			"%w: latest fetched block is higher than actual latest block: %d > %d",
			ErrBlockMismatch, latestHandled.Number, actualLatest.Number,
		)

	default:
		return nil, nil
	}

	rangeSize := uint32(blocksRange.End - blocksRange.Start + 1)
	if rangeSize <= maxNumBlocks {
		return &blocksRange, nil
	}

	return &BlocksRange{blocksRange.Start, blocksRange.Start + types.BlockNumber(maxNumBlocks-1)}, nil
}

func (br *BlockRef) Equals(other *BlockRef) bool {
	if br == nil || other == nil {
		return br == nil && other == nil
	}

	return br.ShardId == other.ShardId && br.Hash == other.Hash && br.Number == other.Number
}

// ValidateDescendant verifies if a given descendant BlockRef is valid relative to the current BlockRef.
func (br *BlockRef) ValidateDescendant(descendant BlockRef) error {
	switch {
	case br == nil:
		return nil

	case descendant.ShardId != br.ShardId:
		return fmt.Errorf(
			"%w: [hash=%s] shard mismatch: expected=%d, got=%d",
			ErrBlockMismatch, descendant.Hash, br.ShardId, descendant.ShardId,
		)

	case descendant.Number <= br.Number:
		return fmt.Errorf(
			"%w: [hash=%s] block number mismatch: expected>%d, got=%d",
			ErrBlockMismatch, descendant.Hash, br.Number, descendant.Number,
		)

	default:
		return nil
	}
}

// ValidateNext ensures that the given child block is a valid subsequent block of the current BlockRef.
func (br *BlockRef) ValidateNext(child *Block) error {
	childRef := BlockToRef(child)
	if err := br.ValidateDescendant(childRef); err != nil {
		return err
	}

	switch {
	case br == nil:
		return nil

	case child.Number != br.Number+1:
		return fmt.Errorf(
			"%w: [hash=%s] block number mismatch: expected=%d, got=%d",
			ErrBlockMismatch, child.Hash, br.Number+1, child.Number,
		)

	case child.ParentHash != br.Hash:
		return fmt.Errorf(
			"%w: [hash=%s] parent hash mismatch: expected=%s, got=%s",
			ErrBlockMismatch, child.Hash, br.Hash, child.ParentHash,
		)

	default:
		return nil
	}
}

func GetParentRef(block *Block) *BlockRef {
	if block.Number == 0 || block.ParentHash.Empty() {
		return nil
	}
	return &BlockRef{
		ShardId: block.ShardId,
		Number:  block.Number - 1,
		Hash:    block.ParentHash,
	}
}

type BlockId struct {
	ShardId types.ShardId
	Hash    common.Hash
}

func NewBlockId(shardId types.ShardId, hash common.Hash) BlockId {
	return BlockId{shardId, hash}
}

func IdFromBlock(block *Block) BlockId {
	return BlockId{block.ShardId, block.Hash}
}

func ChildBlockIds(mainShardBlock *Block) ([]BlockId, error) {
	if mainShardBlock == nil {
		return nil, errors.New("mainShardBlock cannot be nil")
	}

	if mainShardBlock.ShardId != types.MainShardId {
		return nil, fmt.Errorf("mainShardBlock is not from the main shard: %d", mainShardBlock.ShardId)
	}

	blockIds := make([]BlockId, 0, len(mainShardBlock.ChildBlocks))

	for i, childHash := range mainShardBlock.ChildBlocks {
		if childHash.Empty() {
			continue
		}

		shardId := types.ShardId(i + 1)
		blockId := NewBlockId(shardId, childHash)
		blockIds = append(blockIds, blockId)
	}

	return blockIds, nil
}

func (bk BlockId) Bytes() []byte {
	key := make([]byte, 4+common.HashSize)
	binary.LittleEndian.PutUint32(key[:4], uint32(bk.ShardId))
	copy(key[4:], bk.Hash.Bytes())
	return key
}

func (bk BlockId) String() string {
	return hex.EncodeToString(bk.Bytes())
}

type PrunedBlock struct {
	ShardId       types.ShardId
	BlockNumber   types.BlockNumber
	Timestamp     uint64
	PrevBlockHash common.Hash
	Transactions  []PrunedTransaction
}

func NewPrunedBlock(block *Block) *PrunedBlock {
	return &PrunedBlock{
		ShardId:       block.ShardId,
		BlockNumber:   block.Number,
		Timestamp:     block.DbTimestamp,
		PrevBlockHash: block.ParentHash,
		Transactions:  BlockTransactions(block),
	}
}

type PrunedTransaction struct {
	Flags    types.TransactionFlags
	Seqno    hexutil.Uint64
	From     types.Address
	To       types.Address
	BounceTo types.Address
	RefundTo types.Address
	Value    types.Value
	Data     hexutil.Bytes
}

func BlockTransactions(block *Block) []PrunedTransaction {
	transactions := make([]PrunedTransaction, len(block.Transactions))
	for idx, transaction := range block.Transactions {
		transactions[idx] = NewTransaction(transaction)
	}
	return transactions
}

func NewTransaction(transaction *jsonrpc.RPCInTransaction) PrunedTransaction {
	return PrunedTransaction{
		Flags:    transaction.Flags,
		Seqno:    transaction.Seqno,
		From:     transaction.From,
		To:       transaction.To,
		BounceTo: transaction.BounceTo,
		RefundTo: transaction.RefundTo,
		Value:    transaction.Value,
		Data:     transaction.Data,
	}
}

type DataProofs [][]byte

type ProposalData struct {
	BatchId             BatchId
	DataProofs          DataProofs
	OldProvedStateRoot  common.Hash
	NewProvedStateRoot  common.Hash
	FirstBlockFetchedAt time.Time
}

func NewProposalData(
	batchId BatchId,
	dataProofs DataProofs,
	oldProvedStateRoot common.Hash,
	newProvedStateRoot common.Hash,
	mainBlockFetchedAt time.Time,
) *ProposalData {
	return &ProposalData{
		BatchId:             batchId,
		DataProofs:          dataProofs,
		OldProvedStateRoot:  oldProvedStateRoot,
		NewProvedStateRoot:  newProvedStateRoot,
		FirstBlockFetchedAt: mainBlockFetchedAt,
	}
}
