package v1

import (
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/hexutil"
	coreTypes "github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types/proto"
)

func uint256ToProtoUint256(u coreTypes.Uint256) *proto.Uint256 {
	return &proto.Uint256{
		WordParts: u[:],
	}
}

func protoUint256ToUint256(pb *proto.Uint256) coreTypes.Uint256 {
	var u coreTypes.Uint256
	copy(u[:], pb.WordParts)
	return u
}

func ConvertToProto(batch *types.PrunedBatch) *proto.Batch {
	var (
		lastTs       uint64
		totalTxCount uint64
		protoBlocks  = make([]*proto.BlobBlock, 0, len(batch.Blocks))
	)
	for _, l2Blk := range batch.Blocks {
		b := &proto.BlobBlock{
			ShardId:       uint32(l2Blk.ShardId),
			BlockNumber:   l2Blk.BlockNumber.Uint64(),
			Timestamp:     l2Blk.Timestamp,
			PrevBlockHash: l2Blk.PrevBlockHash.Bytes(),
		}
		for _, l2Tx := range l2Blk.Transactions {
			tx := &proto.BlobTransaction{
				Flags: uint32(l2Tx.Flags.Bits),
				SeqNo: l2Tx.Seqno.Uint64(),
				AddrFrom: &proto.Address{
					AddressBytes: l2Tx.From.Bytes(),
				},
				AddrTo: &proto.Address{
					AddressBytes: l2Tx.To.Bytes(),
				},
				Value: uint256ToProtoUint256(*l2Tx.Value.Uint256),
				Data:  []byte(l2Tx.Data),
			}

			if !l2Tx.RefundTo.IsEmpty() && !l2Tx.From.Equal(l2Tx.RefundTo) {
				tx.AddrRefundTo = &proto.Address{AddressBytes: l2Tx.RefundTo.Bytes()}
			}
			if !l2Tx.BounceTo.IsEmpty() && !l2Tx.From.Equal(l2Tx.BounceTo) {
				tx.AddrBounceTo = &proto.Address{AddressBytes: l2Tx.BounceTo.Bytes()}
			}
			b.Transactions = append(b.Transactions, tx)
		}
		lastTs = max(lastTs, b.Timestamp)
		totalTxCount += uint64(len(b.Transactions))
		protoBlocks = append(protoBlocks, b)
	}

	return &proto.Batch{
		BatchId:            batch.BatchId.String(),
		LastBlockTimestamp: lastTs,
		Blocks:             protoBlocks,
	}
}

func ConvertFromProto(batch *proto.Batch) (*types.PrunedBatch, error) {
	blocks := make([]*types.PrunedBlock, 0, len(batch.Blocks))
	for _, pblk := range batch.Blocks {
		b := &types.PrunedBlock{
			ShardId:       coreTypes.ShardId(pblk.ShardId),
			BlockNumber:   coreTypes.BlockNumber(pblk.BlockNumber),
			Timestamp:     pblk.Timestamp,
			PrevBlockHash: common.BytesToHash(pblk.PrevBlockHash),
		}
		for _, ptx := range pblk.Transactions {
			tx := types.PrunedTransaction{
				Flags: coreTypes.NewTransactionFlagsFromBits(uint8(ptx.GetFlags())),
				Seqno: hexutil.Uint64(ptx.GetSeqNo()),
				From:  coreTypes.BytesToAddress(ptx.AddrFrom.AddressBytes),
				To:    coreTypes.BytesToAddress(ptx.AddrTo.AddressBytes),
				Data:  ptx.GetData(),
			}
			pValue := protoUint256ToUint256(ptx.Value)
			tx.Value = coreTypes.Value{Uint256: &pValue}
			if ptx.AddrRefundTo != nil {
				tx.RefundTo = coreTypes.BytesToAddress(ptx.AddrFrom.AddressBytes)
			}
			if ptx.AddrBounceTo != nil {
				tx.BounceTo = coreTypes.BytesToAddress(ptx.AddrFrom.AddressBytes)
			}
			b.Transactions = append(b.Transactions, tx)
		}
		blocks = append(blocks, b)
	}

	var id types.BatchId
	if err := id.UnmarshalText([]byte(batch.BatchId)); err != nil {
		return nil, err
	}
	return &types.PrunedBatch{BatchId: id, Blocks: blocks}, nil
}
