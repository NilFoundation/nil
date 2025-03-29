package workload

import (
	"context"
	"math/big"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/stresser/core"
)

type SendRequests struct {
	WorkloadBase `yaml:",inline"`
	GasRange     Range `yaml:"gasRange"`
	RequestsNum  int   `yaml:"requestsNum"`
	addresses    []types.Address
}

func (w *SendRequests) Init(ctx context.Context, client *core.Helper, args *WorkloadParams) error {
	w.WorkloadBase.Init(ctx, client, args)
	w.addresses = make([]types.Address, 0, w.RequestsNum)
	for _, cntr := range args.Contracts {
		w.addresses = append(w.addresses, cntr.Address)
	}
	w.logger = logging.NewLogger("send_requests")
	return nil
}

func (w *SendRequests) Run(ctx context.Context, args *RunParams) ([]*core.Transaction, error) {
	params := &core.TxParams{FeePack: types.NewFeePackFromGas(100_000_000)}
	for _, contract := range w.params.Contracts {
		n := getNumForGasConsumer(w.GasRange.RandomValue())
		tx, err := w.client.Call(contract, "sendRequests", params, w.addresses, big.NewInt(int64(n)))
		if err != nil {
			w.logger.Error().Err(err).Msg("failed to call sendRequests")
		} else {
			w.AddTx(tx)
		}
	}
	return w.txs, nil
}
