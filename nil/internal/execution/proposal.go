package execution

import (
	"fmt"
	"slices"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/mpt"
	"github.com/NilFoundation/nil/nil/internal/serialization"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/ethereum/go-ethereum/rlp"
)

type ParentBlock struct {
	ShardId types.ShardId
	Block   *types.Block

	TxnTrie *TransactionTrie

	txnTrieHolder mpt.InMemHolder
}

type ParentBlockSerializable struct {
	ShardId       types.ShardId
	TxnTrieHolder *serialization.MapHolder
	Block         *types.Block
}

type InternalTxnReference struct {
	ParentBlockIndex uint32
	TxnIndex         types.TransactionIndex
}

type Proposal struct {
	PrevBlockId     types.BlockNumber   `json:"prevBlockId"`
	PrevBlockHash   common.Hash         `json:"prevBlockHash"`
	PatchLevel      uint32              `json:"patchLevel"`
	RollbackCounter uint32              `json:"rollbackCounter"`
	CollatorState   types.CollatorState `json:"collatorState"`
	MainShardHash   common.Hash         `json:"mainShardHash"`
	ShardHashes     []common.Hash       `json:"shardHashes"`

	InternalTxns []*types.Transaction `json:"internalTxns"`
	ExternalTxns []*types.Transaction `json:"externalTxns"`
	ForwardTxns  []*types.Transaction `json:"forwardTxns"`
}

type ProposalSerializable struct {
	PrevBlockId   types.BlockNumber
	PrevBlockHash common.Hash
	BlockHash     common.Hash

	PatchLevel      uint32
	RollbackCounter uint32

	CollatorState types.CollatorState
	MainShardHash common.Hash
	ShardHashes   []common.Hash

	ParentBlocks []*ParentBlockSerializable

	InternalTxnRefs []*InternalTxnReference
	ForwardTxnRefs  []*InternalTxnReference

	ExternalTxns []*types.Transaction

	// SpecialTxns are internal transactions produced by the collator. They appear only on the main shard.
	SpecialTxns []*types.Transaction
}

func (p *ProposalSerializable) UnmarshalNil(buf []byte) error {
	return rlp.DecodeBytes(buf, p)
}

func (p ProposalSerializable) MarshalNil() ([]byte, error) {
	return rlp.EncodeToBytes(&p)
}

func NewParentBlock(shardId types.ShardId, block *types.Block) *ParentBlock {
	holder := mpt.NewInMemHolder()
	return &ParentBlock{
		ShardId:       shardId,
		Block:         block,
		TxnTrie:       NewTransactionTrie(mpt.NewMPTFromMap(holder)),
		txnTrieHolder: holder,
	}
}

func NewParentBlockFromSerializable(b *ParentBlockSerializable) (*ParentBlock, error) {
	holder := mpt.InMemHolder(b.TxnTrieHolder.ToMap())
	trie := NewTransactionTrie(mpt.NewMPTFromMap(holder))
	if err := trie.SetRootHash(b.Block.OutTransactionsRoot); err != nil {
		return nil, err
	}
	return &ParentBlock{
		ShardId:       b.ShardId,
		Block:         b.Block,
		TxnTrie:       trie,
		txnTrieHolder: holder,
	}, nil
}

func (pb *ParentBlock) ToSerializable() *ParentBlockSerializable {
	return &ParentBlockSerializable{
		Block:         pb.Block,
		TxnTrieHolder: serialization.NewMapHolder(pb.txnTrieHolder),
	}
}

func SplitTransactions(
	transactions []*types.Transaction,
	f func(t *types.Transaction) bool,
) (a, b []*types.Transaction) {
	if pos := slices.IndexFunc(transactions, f); pos != -1 {
		return transactions[:pos], transactions[pos:]
	}

	return transactions, nil
}

// SplitInTransactions splits incoming transactions in the block into internal and external ones.
// Internal transactions come before the external ones.
func SplitInTransactions(transactions []*types.Transaction) (internal, external []*types.Transaction) {
	return SplitTransactions(transactions, func(t *types.Transaction) bool {
		return t.IsExternal()
	})
}

// SplitOutTransactions splits outgoing transactions in the block into forwarded and generated ones.
// Forwarded transactions come before the generated ones.
func SplitOutTransactions(
	transactions []*types.Transaction,
	shardId types.ShardId,
) (forwarded, generated []*types.Transaction) {
	return SplitTransactions(transactions, func(t *types.Transaction) bool {
		return t.From.ShardId() == shardId
	})
}

func convertTxnRefs(refs []*InternalTxnReference, parentBlocks []*ParentBlock) ([]*types.Transaction, error) {
	res := make([]*types.Transaction, len(refs))
	for i, ref := range refs {
		if ref.ParentBlockIndex >= uint32(len(parentBlocks)) {
			return nil, fmt.Errorf("invalid parent block index %d", ref.ParentBlockIndex)
		}

		pb := parentBlocks[ref.ParentBlockIndex]
		txn, err := pb.TxnTrie.Fetch(ref.TxnIndex)
		if err != nil {
			return nil, fmt.Errorf(
				"faulty transaction %d in block (%s, %s): %w", ref.TxnIndex, pb.ShardId, pb.Block.Id, err)
		}
		res[i] = txn
	}
	return res, nil
}

func ConvertProposal(proposal *ProposalSerializable) (*Proposal, error) {
	parentBlocks := make([]*ParentBlock, len(proposal.ParentBlocks))
	for i, pb := range proposal.ParentBlocks {
		converted, err := NewParentBlockFromSerializable(pb)
		if err != nil {
			return nil, fmt.Errorf("invalid parent block: %w", err)
		}
		parentBlocks[i] = converted
	}

	internalTxns, err := convertTxnRefs(proposal.InternalTxnRefs, parentBlocks)
	if err != nil {
		return nil, fmt.Errorf("invalid internal transactions: %w", err)
	}
	forwardTxns, err := convertTxnRefs(proposal.ForwardTxnRefs, parentBlocks)
	if err != nil {
		return nil, fmt.Errorf("invalid forward transactions: %w", err)
	}

	return &Proposal{
		PrevBlockId:     proposal.PrevBlockId,
		PrevBlockHash:   proposal.PrevBlockHash,
		PatchLevel:      proposal.PatchLevel,
		RollbackCounter: proposal.RollbackCounter,
		CollatorState:   proposal.CollatorState,
		MainShardHash:   proposal.MainShardHash,
		ShardHashes:     proposal.ShardHashes,

		// todo: special txns should be validated
		InternalTxns: append(proposal.SpecialTxns, internalTxns...),
		ExternalTxns: proposal.ExternalTxns,
		ForwardTxns:  forwardTxns,
	}, nil
}
