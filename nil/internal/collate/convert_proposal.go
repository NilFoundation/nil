package collate

import (
	"context"
	"fmt"

	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/types"
)

func convertTxnRefs(ctx context.Context,
	tx db.RoTx,
	shardId types.ShardId,
	refs []*execution.InternalTxnReference,
	parentBlocks []*execution.ParentBlock,
) ([]*types.Transaction, error) {
	res := make([]*types.Transaction, len(refs))
	for i, ref := range refs {
		if ref.ParentBlockIndex >= uint32(len(parentBlocks)) {
			return nil, fmt.Errorf("invalid parent block index %d", ref.ParentBlockIndex)
		}

		pb := parentBlocks[ref.ParentBlockIndex]
		relayerReader := NewRelayerReader(pb.ShardId, pb.Block.Id)
		msg, err := relayerReader.GetMessageById(ctx, tx, shardId, uint64(ref.TxnIndex))
		if err != nil {
			return nil, fmt.Errorf(
				"failed to get transaction %d for shard %d in block (%s, %s): %w",
				ref.TxnIndex, shardId, pb.ShardId, pb.Block.Id, err)
		}
		txn := msg.ToTransaction()
		res[i] = txn
	}
	return res, nil
}

func ConvertProposal(
	ctx context.Context,
	txFabric db.DB,
	shardId types.ShardId,
	proposal *execution.ProposalSSZ,
) (*execution.Proposal, error) {
	tx, err := txFabric.CreateRoTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create read-only transaction: %w", err)
	}
	defer tx.Rollback()

	parentBlocks := make([]*execution.ParentBlock, len(proposal.ParentBlocks))
	for i, pb := range proposal.ParentBlocks {
		converted, err := execution.NewParentBlockFromSSZ(pb)
		if err != nil {
			return nil, fmt.Errorf("invalid parent block: %w", err)
		}
		parentBlocks[i] = converted
	}

	internalTxns, err := convertTxnRefs(ctx, tx, shardId, proposal.InternalTxnRefs, parentBlocks)
	if err != nil {
		return nil, fmt.Errorf("invalid internal transactions: %w", err)
	}
	forwardTxns, err := convertTxnRefs(ctx, tx, shardId, proposal.ForwardTxnRefs, parentBlocks)
	if err != nil {
		return nil, fmt.Errorf("invalid forward transactions: %w", err)
	}

	// specialLen := len(proposal.SpecialTxns)
	// if specialLen > 0 {
	// 	// Move indices to take into account the special transactions
	// 	for _, tx := range internalTxns {
	// 		tx.TxId += types.TransactionIndex(specialLen)
	// 	}
	// }

	return &execution.Proposal{
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
