package execution

import (
	"fmt"
	"slices"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/sszx"
	"github.com/NilFoundation/nil/nil/internal/mpt"
	"github.com/NilFoundation/nil/nil/internal/types"
)

type ParentBlock struct {
	ShardId types.ShardId
	Block   *types.Block

	TxnTrie *TransactionTrie

	txnTrieHolder mpt.InMemHolder
}

type ParentBlockSSZ struct {
	ShardId       types.ShardId
	TxnTrieHolder *sszx.MapHolder
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
	MainChainHash   common.Hash         `json:"mainChainHash"`
	ShardHashes     []common.Hash       `json:"shardHashes"`

	InternalTxns []*types.Transaction `json:"internalTxns"`
	ExternalTxns []*types.Transaction `json:"externalTxns"`
	ForwardTxns  []*types.Transaction `json:"forwardTxns"`
}

type ProposalSSZ struct {
	PrevBlockId   types.BlockNumber
	PrevBlockHash common.Hash

	PatchLevel      uint32
	RollbackCounter uint32

	CollatorState types.CollatorState
	MainChainHash common.Hash
	ShardHashes   []common.Hash `ssz-max:"4096"`

	ParentBlocks []*ParentBlockSSZ `ssz-max:"1024"`

	InternalTxnRefs []*InternalTxnReference `ssz-max:"4096"`
	ForwardTxnRefs  []*InternalTxnReference `ssz-max:"4096"`

	ExternalTxns []*types.Transaction `ssz-max:"4096"`

	// SpecialTxns are internal transactions produced by the collator. They appear only on the main shard.
	SpecialTxns []*types.Transaction `ssz-max:"4096"`
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

func NewParentBlockFromSSZ(b *ParentBlockSSZ) (*ParentBlock, error) {
	holder := mpt.InMemHolder(b.TxnTrieHolder.ToMap())
	if err := mpt.ValidateHolder(holder); err != nil {
		return nil, err
	}

	trie := NewTransactionTrie(mpt.NewMPTFromMap(holder))
	trie.SetRootHash(b.Block.OutTransactionsRoot)
	return &ParentBlock{
		ShardId:       b.ShardId,
		Block:         b.Block,
		TxnTrie:       trie,
		txnTrieHolder: holder,
	}, nil
}

func (pb *ParentBlock) ToSerializable() *ParentBlockSSZ {
	return &ParentBlockSSZ{
		Block:         pb.Block,
		TxnTrieHolder: sszx.NewMapHolder(pb.txnTrieHolder),
	}
}

func SplitTransactions(transactions []*types.Transaction, f func(t *types.Transaction) bool) (a, b []*types.Transaction) {
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
func SplitOutTransactions(transactions []*types.Transaction, shardId types.ShardId) (forwarded, generated []*types.Transaction) {
	return SplitTransactions(transactions, func(t *types.Transaction) bool {
		return t.From.ShardId() == shardId
	})
}

func ConvertTxnRefs(refs []*InternalTxnReference, parentBlocks []*ParentBlock) ([]*types.Transaction, error) {
	res := make([]*types.Transaction, len(refs))
	for i, ref := range refs {
		if ref.ParentBlockIndex >= uint32(len(parentBlocks)) {
			return nil, fmt.Errorf("invalid parent block index %d", ref.ParentBlockIndex)
		}

		pb := parentBlocks[ref.ParentBlockIndex]
		txn, err := pb.TxnTrie.Fetch(ref.TxnIndex)
		if err != nil {
			return nil, fmt.Errorf("faulty transaction %d in block (%s, %s): %w", ref.TxnIndex, pb.ShardId, pb.Block.Id, err)
		}
		res[i] = txn
	}
	return res, nil
}

func ConvertProposal(proposal *ProposalSSZ) (*Proposal, error) {
	parentBlocks := make([]*ParentBlock, len(proposal.ParentBlocks))
	for i, pb := range proposal.ParentBlocks {
		converted, err := NewParentBlockFromSSZ(pb)
		if err != nil {
			return nil, fmt.Errorf("invalid parent block: %w", err)
		}
		parentBlocks[i] = converted
	}

	internalTxns, err := ConvertTxnRefs(proposal.InternalTxnRefs, parentBlocks)
	if err != nil {
		return nil, fmt.Errorf("invalid internal transactions: %w", err)
	}
	forwardTxns, err := ConvertTxnRefs(proposal.ForwardTxnRefs, parentBlocks)
	if err != nil {
		return nil, fmt.Errorf("invalid forward transactions: %w", err)
	}

	return &Proposal{
		PrevBlockId:     proposal.PrevBlockId,
		PrevBlockHash:   proposal.PrevBlockHash,
		PatchLevel:      proposal.PatchLevel,
		RollbackCounter: proposal.RollbackCounter,
		CollatorState:   proposal.CollatorState,
		MainChainHash:   proposal.MainChainHash,
		ShardHashes:     proposal.ShardHashes,

		// todo: special txns should be validated
		InternalTxns: append(proposal.SpecialTxns, internalTxns...),
		ExternalTxns: proposal.ExternalTxns,
		ForwardTxns:  forwardTxns,
	}, nil
}
