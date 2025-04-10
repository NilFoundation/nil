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
	AsyncCall    bool  `yaml:"asyncCall"`
	addresses    []types.Address
	mathodName   string
}

func (w *SendRequests) Init(ctx context.Context, client *core.Helper, args *WorkloadParams) error {
	w.WorkloadBase.Init(ctx, client, args)
	w.addresses = make([]types.Address, 0, w.RequestsNum)
	for range w.RequestsNum {
		w.addresses = append(w.addresses, w.getRandomContract().Address)
	}
	w.logger = logging.NewLogger("send_requests")
	w.mathodName = "sendRequests"
	if w.AsyncCall {
		w.mathodName = "asyncCalls"
	}
	return nil
}

func (w *SendRequests) Run(ctx context.Context, args *RunParams) error {
	params := &core.TxParams{FeePack: types.NewFeePackFromGas(100_000_000)}
	for i := range w.Iterations {
		contract := w.getContract(i)
		n := getNumForGasConsumer(w.GasRange.RandomValue())
		if _, err := w.client.Call(contract, w.mathodName, params, w.addresses, big.NewInt(int64(n))); err != nil {
			w.logger.Error().Err(err).Msg("failed to call sendRequests")
		}
	}
	return nil
}
