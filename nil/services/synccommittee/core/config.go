package core

import (
	"github.com/NilFoundation/nil/nil/internal/telemetry"
	"github.com/NilFoundation/nil/nil/services/synccommittee/core/feeupdater"
	"github.com/NilFoundation/nil/nil/services/synccommittee/core/fetching"
	"github.com/NilFoundation/nil/nil/services/synccommittee/core/rollupcontract"
)

const (
	DefaultTaskRpcEndpoint = "tcp://127.0.0.1:8530"
)

type Config struct {
	RpcEndpoint               string                           `yaml:"endpoint,omitempty"`
	TaskListenerRpcEndpoint   string                           `yaml:"ownEndpoint,omitempty"`
	AggregatorConfig          fetching.AggregatorConfig        `yaml:",inline"`
	ProposerParams            ProposerConfig                   `yaml:"-"`
	ContractWrapperConfig     rollupcontract.WrapperConfig     `yaml:",inline"`
	L1FeeUpdateConfig         feeupdater.Config                `yaml:",inline"`
	L1FeeUpdateContractConfig feeupdater.ContractWrapperConfig `yaml:",inline"`
	Telemetry                 *telemetry.Config                `yaml:",inline"`
}

func NewDefaultConfig() *Config {
	return &Config{
		RpcEndpoint:             "tcp://127.0.0.1:8529",
		TaskListenerRpcEndpoint: DefaultTaskRpcEndpoint,
		AggregatorConfig:        fetching.NewDefaultAggregatorConfig(),
		ProposerParams:          NewDefaultProposerConfig(),
		ContractWrapperConfig:   rollupcontract.NewDefaultWrapperConfig(),
		L1FeeUpdateConfig:       feeupdater.DefaultConfig(),
		Telemetry: &telemetry.Config{
			ServiceName: "sync_committee",
		},
	}
}
