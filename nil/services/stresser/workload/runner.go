package workload

import (
	"context"
	"time"

	"github.com/NilFoundation/nil/nil/common/logging"
)

type Runner struct {
	Workload Workload
	tasks    chan RunParams
	logger   logging.Logger
}

func NewRunner(workload Workload) *Runner {
	r := &Runner{
		Workload: workload,
		tasks:    make(chan RunParams),
		logger:   logging.NewLogger(workload.GetName()),
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
			startTm := time.Now()
			err := r.Workload.Run(ctx, &p)
			if err != nil {
				r.logger.Error().Err(err).Msg("Error running workload")
			}
			r.logger.Info().Msgf("Iteration done in %.2fs for %s", time.Since(startTm).Seconds(), r.Workload.GetName())
		}
	}
}
