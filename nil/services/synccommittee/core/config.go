package core

import (
	"time"

	"github.com/NilFoundation/nil/nil/internal/telemetry"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/srv"
)

const (
	DefaultTaskRpcEndpoint = "tcp://127.0.0.1:8530"
)

type Config struct {
	srv.Config

	RpcEndpoint             string
	TaskListenerRpcEndpoint string
	PollingDelay            time.Duration
	ProposerParams          *ProposerParams
	Telemetry               *telemetry.Config
}

func NewDefaultConfig() *Config {
	return &Config{
		Config:                  srv.DefaultConfig(),
		RpcEndpoint:             "tcp://127.0.0.1:8529",
		TaskListenerRpcEndpoint: DefaultTaskRpcEndpoint,
		PollingDelay:            time.Second,
		ProposerParams:          NewDefaultProposerParams(),
		Telemetry: &telemetry.Config{
			ServiceName: "sync_committee",
		},
	}
}
