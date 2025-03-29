package workload

import (
	"context"

	"github.com/NilFoundation/nil/nil/internal/telemetry"
	"github.com/NilFoundation/nil/nil/internal/telemetry/telattr"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/stresser/core"
	"go.opentelemetry.io/otel/metric"
)

const (
	batchSize = 20
)

var (
	meter               = telemetry.NewMeter("stresser")
	blockTxsNum         = telemetry.Int64Gauge(meter, "block_txs_num")
	blockExternalTxsNum = telemetry.Int64Gauge(meter, "block_external_txs_num")
	blockInternalTxsNum = telemetry.Int64Gauge(meter, "block_internal_txs_num")
	blockResponseTxsNum = telemetry.Int64Gauge(meter, "block_response_txs_num")
	blockRefundTxsNum   = telemetry.Int64Gauge(meter, "block_refund_txs_num")
	blockBaseFee        = telemetry.Int64Gauge(meter, "block_base_fee")
)

// BlockchainMetrics is a workload that collects metrics about the blockchain.
// It uses GetBlock and GetBlocksRange to get the latest block and a range of blocks.
type BlockchainMetrics struct {
	WorkloadBase `yaml:",inline"`
	lastBlocks   []types.BlockNumber
	options      []metric.MeasurementOption
}

type BlockInfo struct {
	Block *types.Block
	Tps   float64
}

func (w *BlockchainMetrics) Init(ctx context.Context, client *core.Helper, args *WorkloadParams) error {
	w.WorkloadBase.Init(ctx, client, args)
	for shardId := range args.NumShards {
		w.options = append(w.options, telattr.With(telattr.ShardId(types.ShardId(shardId))))
	}
	w.lastBlocks = make([]types.BlockNumber, args.NumShards)
	return nil
}

func (w *BlockchainMetrics) Run(ctx context.Context, args *RunParams) ([]*core.Transaction, error) {
	for shard := range w.params.NumShards {
		block, err := w.client.Client.GetBlock(ctx, types.ShardId(shard), "latest", true)
		if err != nil {
			w.logger.Error().Err(err).Msg("failed to get last block")
			continue
		}
		if w.lastBlocks[shard] == 0 {
			w.lastBlocks[shard] = block.Number
			continue
		}
		if block != nil && w.lastBlocks[shard] == block.Number {
			continue
		}
		blocks, err := w.client.Client.GetBlocksRange(ctx, types.ShardId(shard), w.lastBlocks[shard], block.Number-1,
			true, batchSize)
		if err != nil {
			w.logger.Error().Err(err).Msg("failed to get block range")
			continue
		}
		blocks = append(blocks, block)
		w.lastBlocks[shard] = block.Number

		for _, b := range blocks {
			externalTxsNum := int64(0)
			internalTxsNum := int64(0)
			responseTxsNum := int64(0)
			refundTxsNum := int64(0)

			for _, tx := range b.Transactions {
				if tx.Flags.IsInternal() {
					internalTxsNum++
				} else {
					externalTxsNum++
				}
				if tx.Flags.IsResponse() {
					responseTxsNum++
				}
				if tx.Flags.IsRefund() {
					refundTxsNum++
				}
			}
			blockTxsNum.Record(ctx, int64(len(b.Transactions)), w.options[b.ShardId])
			blockExternalTxsNum.Record(ctx, externalTxsNum, w.options[b.ShardId])
			blockInternalTxsNum.Record(ctx, internalTxsNum, w.options[b.ShardId])
			blockResponseTxsNum.Record(ctx, responseTxsNum, w.options[b.ShardId])
			blockRefundTxsNum.Record(ctx, refundTxsNum, w.options[b.ShardId])
			blockBaseFee.Record(ctx, int64(b.BaseFee.Uint64()), w.options[b.ShardId])
		}
	}

	return nil, nil
}
