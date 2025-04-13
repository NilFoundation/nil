package workload

import (
	"context"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/stresser/core"
)

const (
	defaultBlockRange = 100
	defaultBatchSize  = 20
)

type BlockRange struct {
	WorkloadBase `yaml:",inline"`
	Range        int `yaml:"range"`
	BatchSize    int `yaml:"batchSize"`
}

func (w *BlockRange) Init(ctx context.Context, client *core.Helper, args *WorkloadParams) error {
	w.WorkloadBase.Init(ctx, client, args)
	if w.Range == 0 {
		w.Range = defaultBlockRange
	}
	if w.BatchSize == 0 {
		w.BatchSize = defaultBatchSize
	}
	w.logger = logging.NewLogger("block_range")
	return nil
}

func (w *BlockRange) Run(ctx context.Context, args *RunParams) error {
	for shard := range w.params.NumShards {
		block, err := w.client.Client.GetBlock(ctx, types.ShardId(shard), "latest", true)
		if err != nil {
			w.logger.Error().Err(err).Msg("failed to get last block")
			continue
		}
		startBlock := types.BlockNumber(0)
		endBlockNumber := block.Number + 1
		if endBlockNumber-startBlock > types.BlockNumber(w.Range) {
			startBlock = endBlockNumber - types.BlockNumber(w.Range)
		}
		_, err = w.client.Client.GetBlocksRange(ctx, types.ShardId(shard), startBlock, block.Number, true, w.BatchSize)
		if err != nil {
			w.logger.Error().Err(err).Msg("failed to get block range")
			continue
		}
	}

	return nil
}
