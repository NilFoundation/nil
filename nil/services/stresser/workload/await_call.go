package workload

import (
	"context"
	"math/rand"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/stresser/core"
)

// AwaitCall is a workload that calculates factorial by recursively calling factorial function via awaitCall.
// It is quite expensive since it produces a lot of transactions. The intensity of the workload can be controlled
// by setting N and ContractsNumToSend to a smaller value.
type AwaitCall struct {
	WorkloadBase `yaml:",inline"`
	N            uint32 `yaml:"n"` // Depth of the factorial
}

func (w *AwaitCall) Init(ctx context.Context, client *core.Helper, args *WorkloadParams) error {
	w.WorkloadBase.Init(ctx, client, args)
	w.logger = logging.NewLogger("await_call")
	return nil
}

func (w *AwaitCall) Run(ctx context.Context, args *RunParams) error {
	options := &core.TxParams{FeePack: types.NewFeePackFromGas(100_000_000)}

	start := rand.Intn(len(w.params.Contracts)) //nolint:gosec
	for i := range w.Iterations {
		contract := w.getContract(start + i)
		var peer types.Address
		if i == 0 {
			peer = w.params.Contracts[len(w.params.Contracts)-1].Address
		} else {
			peer = w.params.Contracts[i-1].Address
		}

		if _, err := w.client.Call(contract, "factorialAwait", options, w.N, peer); err != nil {
			w.logger.Error().Err(err).Msg("failed to call factorialAwait")
		}
	}
	return nil
}
