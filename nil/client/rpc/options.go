package rpc

import (
	"github.com/NilFoundation/nil/nil/common"
)

type config struct {
	retry *common.RetryConfig
}

type Option func(*config)

func RPCRetryConfig(rcfg *common.RetryConfig) Option {
	return func(cfg *config) {
		cfg.retry = rcfg
	}
}
