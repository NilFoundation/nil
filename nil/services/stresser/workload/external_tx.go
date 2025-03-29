package workload

import (
	"context"
	"errors"
	"math/big"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/stresser/core"
)

// ExternalTxs workload sends single external transactions to the network.
// Range specifies how many gas transactions will consume. It will be randomly selected between From and To.
// MinTxsPerIteration specifies the minimum number of transactions to be sent in one iteration.
type ExternalTxs struct {
	WorkloadBase       `yaml:",inline"`
	GasRange           Range `yaml:"gasRange"`
	MinTxsPerIteration int   `yaml:"minTxsPerIteration"`
}

// getNumForGasConsumer calculates the number of iterations (n parameter of `gasConsumer` contract method) for the
// given gas. 529 was calculated experimentally.
func getNumForGasConsumer(gas uint64) uint64 {
	return gas / 529
}

func (w *ExternalTxs) Init(ctx context.Context, client *core.Helper, params *WorkloadParams) error {
	w.WorkloadBase.Init(ctx, client, params)
	if w.GasRange.To < w.GasRange.From {
		return errors.New("GasRange.From should be less than GasRange.To")
	}
	w.logger = logging.NewLogger("external_tx")
	return nil
}

func (w *ExternalTxs) Run(ctx context.Context, args *RunParams) ([]*core.Transaction, error) {
	options := &core.TxParams{FeePack: types.NewFeePackFromGas(100_000_000)}
	sentTxNum := 0
	for sentTxNum < w.MinTxsPerIteration {
		for _, contract := range w.params.Contracts {
			n := getNumForGasConsumer(w.GasRange.RandomValue())
			if tx, err := w.client.Call(contract, "gasConsumer", options, big.NewInt(int64(n))); err != nil {
				w.logger.Error().Err(err).Msg("failed to call contract")
			} else {
				w.AddTx(tx)
			}
			sentTxNum++
		}
	}
	return w.txs, nil
}
