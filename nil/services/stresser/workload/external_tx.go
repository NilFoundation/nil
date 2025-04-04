package workload

import (
	"context"
	"errors"
	"fmt"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/stresser/core"
	"math/big"
	"sync"
)

// ExternalTxs workload sends single external transactions to the network.
// Range specifies how many gas transactions will consume. It will be randomly selected between From and To.
type ExternalTxs struct {
	WorkloadBase `yaml:",inline"`
	GasRange     Range `yaml:"gasRange"`
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

func (w *ExternalTxs) Run(ctx context.Context, args *RunParams) error {
	var batchLimit = len(w.params.Contracts)
	options := &core.TxParams{FeePack: types.NewFeePackFromGas(100_000_000)}
	for i := 0; i < w.Iterations; i += batchLimit {
		wg := &sync.WaitGroup{}
		for j := 0; j < batchLimit && i+j < w.Iterations; j++ {
			wg.Add(1)
			go func(idx int) {
				contract := w.getContract(idx)
				n := getNumForGasConsumer(w.GasRange.RandomValue())
				if _, err := w.client.Call(contract, "gasConsumer", options, big.NewInt(int64(n))); err != nil {
					w.logger.Error().Err(err).Msg("failed to call contract")
				}
				wg.Done()
			}(j)
		}
		wg.Wait()
	}
	return nil
}

func (w *ExternalTxs) Run1(ctx context.Context, args *RunParams) error {
	var batchLimit = len(w.params.Contracts)
	options := &core.TxParams{FeePack: types.NewFeePackFromGas(100_000_000)}
	batch := w.client.Client.CreateBatchRequest()
	batchSize := 0
	for i := range w.Iterations {
		if batchSize >= batchLimit {
			_, err := w.client.Client.BatchCall(ctx, batch)
			if err != nil {
				return fmt.Errorf("failed to execute batch call: %w", err)
			}
			batch = w.client.Client.CreateBatchRequest()
			batchSize = 0
		}
		batchSize++
		contract := w.getContract(i)
		n := getNumForGasConsumer(w.GasRange.RandomValue())

		calldata, err := contract.PackCallData("gasConsumer", big.NewInt(int64(n)))
		if err != nil {
			return fmt.Errorf("failed to pack call data: %w", err)
		}
		_, err = batch.SendExternalTransaction(ctx, calldata, contract.Address, nil, options.FeePack)
		if err != nil {
			return fmt.Errorf("failed to send external transaction: %w", err)
		}
	}

	if batchSize != 0 {
		_, err := w.client.Client.BatchCall(ctx, batch)
		if err != nil {
			return fmt.Errorf("failed to execute batch call: %w", err)
		}
	}
	return nil
}
