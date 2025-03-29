package workload

import (
	"context"
	"fmt"
	"math/rand"
	"sync/atomic"
	"time"

	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/services/stresser/core"
)

type Workload interface {
	Init(ctx context.Context, client *core.Helper, args *WorkloadParams) error
	PreRun(ctx context.Context, args *RunParams)
	Run(ctx context.Context, args *RunParams) ([]*core.Transaction, error)
	TotalTxsNum() int
	CheckIsReady() bool
}

type WorkloadBase struct {
	Interval       time.Duration `yaml:"interval"`
	WaitTxsTimeout time.Duration `yaml:"waitTxsTimeout"`

	params WorkloadParams
	// Time of the workload last start, used to calculate the interval.
	lastStartTm time.Time
	// List of created transactions on each run. We can use local array and return it on each run, but we also need
	// to take into account that we don't need to add transactions if WaitTxsTimeout!=0. Now it is checked in one place
	// instead of checking it in each workload.
	txs []*core.Transaction
	// Total number of transactions sent by the workload
	totalTxsNum atomic.Int64

	client *core.Helper
	logger logging.Logger
}

type RunParams struct{}

type WorkloadParams struct {
	Contracts []*core.Contract
	NumShards int
}

func (w *WorkloadBase) Init(ctx context.Context, client *core.Helper, args *WorkloadParams) {
	w.client = client
	w.params = *args
	w.lastStartTm = time.Now()
}

func (w *WorkloadBase) PreRun(ctx context.Context, args *RunParams) {
	w.txs = nil
}

func (w *WorkloadBase) CheckIsReady() bool {
	res := time.Since(w.lastStartTm) >= w.Interval
	if res {
		w.lastStartTm = time.Now()
	}
	return res
}

func (w *WorkloadBase) AddTx(tx *core.Transaction) {
	if w.shouldWaitTx() {
		tx.Timeout = w.WaitTxsTimeout
		w.txs = append(w.txs, tx)
	}
	w.totalTxsNum.Add(1)
}

func (w *WorkloadBase) TotalTxsNum() int {
	return int(w.totalTxsNum.Load())
}

func (w *WorkloadBase) shouldWaitTx() bool {
	return w.WaitTxsTimeout > 0
}

func GetWorkload(name string) (Workload, error) {
	var wd Workload
	switch name {
	case "external_tx":
		wd = &ExternalTxs{}
	case "await_call":
		wd = &AwaitCall{}
	case "block_range":
		wd = &BlockRange{}
	case "send_requests":
		wd = &SendRequests{}
	case "blockchain_metrics":
		wd = &BlockchainMetrics{}
	case "do_panic":
		wd = &DoPanic{}
	default:
		return nil, fmt.Errorf("unknown workload name: %s", name)
	}
	return wd, nil
}

type Range struct {
	From, To uint64
}

func NewGasRange(from, to uint64) Range {
	check.PanicIfNotf(to > from, "GasRange.From should be less than GasRange.To")
	return Range{From: from, To: to}
}

func (r Range) RandomValue() uint64 {
	gas := r.From
	if r.To != r.From {
		v := rand.Intn(int(r.To - r.From)) //nolint:gosec
		gas = uint64(v) + r.From
	}
	return gas
}
