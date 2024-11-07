package core

import (
	"time"

	"github.com/NilFoundation/nil/nil/internal/telemetry"
)

type Config struct {
	RpcEndpoint      string
	OwnRpcEndpoint   string
	PollingDelay     time.Duration
	GracefulShutdown bool
	ProposerParams   *ProposerParams
	Telemetry        *telemetry.Config
}

func NewDefaultConfig() *Config {
	return &Config{
		RpcEndpoint:      "tcp://127.0.0.1:8529",
		OwnRpcEndpoint:   "tcp://127.0.0.1:8530",
		PollingDelay:     time.Second,
		GracefulShutdown: true,
		ProposerParams:   NewDefaultProposerParams(),
		Telemetry: &telemetry.Config{
			ServiceName: "sync_committee",
		},
	}
}
