package workload

import (
	"context"

	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/stresser/core"
)

type DoPanic struct {
	WorkloadBase `yaml:",inline"`
	ShardId      types.ShardId `yaml:"shardId"`
}

func (w *DoPanic) Init(ctx context.Context, client *core.Helper, params *WorkloadParams) error {
	w.WorkloadBase.Init(ctx, client, params)
	return nil
}

func (w *DoPanic) Run(ctx context.Context, args *RunParams) error {
	_, err := w.client.Client.DoPanicOnShard(ctx, w.ShardId)
	w.logger.Info().Err(err).Msg("do panic")
	return err
}
