package exec

import (
	"errors"
	"fmt"
	"time"
)

var ErrNoDataFound = errors.New("no data found")

type Params struct {
	AutoRefresh     bool
	RefreshInterval time.Duration
}

const (
	RefreshIntervalMinimal = 100 * time.Millisecond
	RefreshIntervalDefault = 5 * time.Second
)

func DefaultExecutorParams() Params {
	return Params{
		AutoRefresh:     false,
		RefreshInterval: RefreshIntervalDefault,
	}
}

func (p Params) Validate() error {
	if p.AutoRefresh && p.RefreshInterval < RefreshIntervalMinimal {
		return fmt.Errorf(
			"refresh interval cannot be less than %s, actual is %s", RefreshIntervalMinimal, p.RefreshInterval)
	}
	return nil
}

func (p Params) GetExecutorParams() *Params {
	return &p
}

type NoRefreshParams struct{}

func (*NoRefreshParams) GetExecutorParams() *Params {
	params := DefaultExecutorParams()
	params.AutoRefresh = false
	return &params
}
