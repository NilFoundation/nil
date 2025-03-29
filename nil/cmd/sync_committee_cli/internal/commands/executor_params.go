package commands

import (
	"errors"
	"fmt"
	"time"

	"github.com/NilFoundation/nil/nil/services/synccommittee/core"
)

var ErrNoDataFound = errors.New("no data found")

type ExecutorParams struct {
	DebugRpcEndpoint string
	AutoRefresh      bool
	RefreshInterval  time.Duration
}

const MinRefreshInterval = 100 * time.Millisecond

func DefaultExecutorParams() *ExecutorParams {
	return &ExecutorParams{
		DebugRpcEndpoint: core.DefaultTaskRpcEndpoint,
		AutoRefresh:      false,
		RefreshInterval:  5 * time.Second,
	}
}

func (p *ExecutorParams) Validate() error {
	if p.AutoRefresh && p.RefreshInterval < MinRefreshInterval {
		return fmt.Errorf(
			"refresh interval cannot be less than %s, actual is %s", MinRefreshInterval, p.RefreshInterval)
	}
	return nil
}
