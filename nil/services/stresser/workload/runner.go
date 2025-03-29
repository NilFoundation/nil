package workload

import (
	"context"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/services/stresser/core"
)

type Runner struct {
	Workload Workload
	tasks    chan RunParams
	newTxs   chan<- []*core.Transaction
	logger   logging.Logger
}

func NewRunner(workload Workload, newTxs chan<- []*core.Transaction) *Runner {
	r := &Runner{
		Workload: workload,
		newTxs:   newTxs,
		tasks:    make(chan RunParams),
		logger:   logging.NewLogger("runner"),
	}
	return r
}

func (r *Runner) StartMainLoop(ctx context.Context) error {
	go r.MainLoop(ctx)
	return nil
}

func (r *Runner) RunWorkload(params RunParams) error {
	if r.Workload.CheckIsReady() {
		r.tasks <- params
	}
	return nil
}

func (r *Runner) MainLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case p := <-r.tasks:
			r.Workload.PreRun(ctx, &p)
			txs, err := r.Workload.Run(ctx, &p)
			if err != nil {
				r.logger.Error().Err(err).Msg("Error running workload")
			} else if len(txs) > 0 {
				r.newTxs <- txs
			}
		}
	}
}
