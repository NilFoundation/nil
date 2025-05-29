package core

import (
	"github.com/NilFoundation/nil/nil/internal/telemetry"
	"github.com/NilFoundation/nil/nil/services/synccommittee/core/feeupdater"
	"github.com/NilFoundation/nil/nil/services/synccommittee/core/fetching"
	"github.com/NilFoundation/nil/nil/services/synccommittee/core/rollupcontract"
)

const (
	DefaultNilRpcEndpoint = "tcp://127.0.0.1:8529"
	DefaultOwnRpcEndpoint = "tcp://127.0.0.1:8530"
)

type Config struct {
	NilRpcEndpoint            string                           `yaml:"endpoint,omitempty"`
	OwnRpcEndpoint            string                           `yaml:"ownEndpoint,omitempty"`
	AggregatorConfig          fetching.AggregatorConfig        `yaml:",inline"`
	ProposerParams            ProposerConfig                   `yaml:",inline"`
	ContractWrapperConfig     rollupcontract.WrapperConfig     `yaml:",inline"`
	L1FeeUpdateConfig         feeupdater.Config                `yaml:",inline"`
	L1FeeUpdateContractConfig feeupdater.ContractWrapperConfig `yaml:",inline"`
	L2BridgeMessengerAddress  string                           `yaml:"l2BridgeMessengerAddress"`
	Telemetry                 *telemetry.Config                `yaml:",inline"`
}

func NewDefaultConfig() *Config {
	return &Config{
		NilRpcEndpoint:        DefaultNilRpcEndpoint,
		OwnRpcEndpoint:        DefaultOwnRpcEndpoint,
		AggregatorConfig:      fetching.NewDefaultAggregatorConfig(),
		ProposerParams:        NewDefaultProposerConfig(),
		ContractWrapperConfig: rollupcontract.NewDefaultWrapperConfig(),
		L1FeeUpdateConfig:     feeupdater.DefaultConfig(),
		Telemetry: &telemetry.Config{
			ServiceName: "sync_committee",
		},
	}
}
