package workload

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/services/stresser/core"
)

type Workload interface {
	Init(ctx context.Context, client *core.Helper, args *WorkloadParams) error
	GetName() string
	PreRun(ctx context.Context, args *RunParams)
	Run(ctx context.Context, args *RunParams) error
	TotalTxsNum() int
	CheckIsReady() bool
}

type WorkloadBase struct {
	Interval   time.Duration `yaml:"interval"`
	Iterations int           `yaml:"iterations"`
	Name       string        `yaml:"name"`

	params WorkloadParams
	// Time of the workload last start, used to calculate the interval.
	lastStartTm time.Time
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
	check.PanicIfNotf(w.Name != "", "workload name should not be empty")
	w.logger = logging.NewLogger(w.Name)
}

func (w *WorkloadBase) GetName() string {
	return w.Name
}

func (w *WorkloadBase) PreRun(ctx context.Context, args *RunParams) {}

func (w *WorkloadBase) CheckIsReady() bool {
	res := time.Since(w.lastStartTm) >= w.Interval
	if res {
		w.lastStartTm = time.Now()
	}
	return res
}

func (w *WorkloadBase) TotalTxsNum() int {
	return int(w.totalTxsNum.Load())
}

func (w *WorkloadBase) getContract(idx int) *core.Contract {
	return w.params.Contracts[idx%len(w.params.Contracts)]
}

func (w *WorkloadBase) getRandomContract() *core.Contract {
	return w.params.Contracts[rand.Intn(len(w.params.Contracts)-1)] //nolint:gosec
}

// batchedFor executes the given function in batches of goroutines, with each batch sized according to the number of
// contracts.
func (w *WorkloadBase) batchedFor(fn func(int)) {
	batchLimit := len(w.params.Contracts)
	for i := 0; i < w.Iterations; i += batchLimit {
		wg := &sync.WaitGroup{}
		for j := 0; j < batchLimit && i+j < w.Iterations; j++ {
			wg.Add(1)
			go func(idx int) {
				fn(idx)
				wg.Done()
			}(j)
		}
		wg.Wait()
	}
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
	case "send_value":
		wd = &SendValue{}
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
