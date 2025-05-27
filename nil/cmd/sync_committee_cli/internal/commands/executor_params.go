package commands

import (
	"errors"
	"fmt"
	"time"
)

var ErrNoDataFound = errors.New("no data found")

type ExecutorParams struct {
	AutoRefresh     bool
	RefreshInterval time.Duration
}

type NoRefresh struct{}

func (*NoRefresh) GetExecutorParams() *ExecutorParams {
	params := ExecutorParamsDefault()
	params.AutoRefresh = false
	return &params
}

const (
	RefreshIntervalMinimal = 100 * time.Millisecond
	RefreshIntervalDefault = 5 * time.Second
)

func ExecutorParamsDefault() ExecutorParams {
	return ExecutorParams{
		AutoRefresh:     false,
		RefreshInterval: RefreshIntervalDefault,
	}
}

func (p ExecutorParams) Validate() error {
	if p.AutoRefresh && p.RefreshInterval < RefreshIntervalMinimal {
		return fmt.Errorf(
			"refresh interval cannot be less than %s, actual is %s", RefreshIntervalMinimal, p.RefreshInterval)
	}
	return nil
}

func (p ExecutorParams) GetExecutorParams() *ExecutorParams {
	return &p
}
