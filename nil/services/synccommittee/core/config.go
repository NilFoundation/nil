package core

import (
	"github.com/NilFoundation/nil/nil/internal/telemetry"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/rollupcontract"
)

const (
	DefaultTaskRpcEndpoint = "tcp://127.0.0.1:8530"
)

type Config struct {
	RpcEndpoint             string
	TaskListenerRpcEndpoint string
	AggregatorConfig        AggregatorConfig
	ProposerParams          ProposerConfig
	ContractWrapperConfig   rollupcontract.WrapperConfig
	Telemetry               *telemetry.Config
}

func NewDefaultConfig() *Config {
	return &Config{
		RpcEndpoint:             "tcp://127.0.0.1:8529",
		TaskListenerRpcEndpoint: DefaultTaskRpcEndpoint,
		AggregatorConfig:        NewDefaultAggregatorConfig(),
		ProposerParams:          NewDefaultProposerConfig(),
		ContractWrapperConfig:   rollupcontract.NewDefaultWrapperConfig(),
		Telemetry: &telemetry.Config{
			ServiceName: "sync_committee",
		},
	}
}
