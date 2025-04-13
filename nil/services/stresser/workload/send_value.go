package workload

import (
	"context"

	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/stresser/core"
)

// SendValue is a workload that sends just a value(no calldata) to a contract.
type SendValue struct {
	WorkloadBase `yaml:",inline"`
}

func (w *SendValue) Init(ctx context.Context, client *core.Helper, params *WorkloadParams) error {
	w.WorkloadBase.Init(ctx, client, params)
	return nil
}

func (w *SendValue) Run(ctx context.Context, args *RunParams) error {
	options := &core.TxParams{
		FeePack: types.NewFeePackFromGas(100_000_000),
		Value:   types.NewValueFromUint64(1000),
	}
	w.batchedFor(func(idx int) {
		contract := w.getContract(idx)
		if _, err := w.client.Call(contract, "", options); err != nil {
			w.logger.Error().Err(err).Msg("failed to call contract")
		}
	})
	return nil
}
