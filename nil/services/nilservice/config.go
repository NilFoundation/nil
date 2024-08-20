package nilservice

import (
	"github.com/NilFoundation/nil/nil/internal/collate"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/network"
	"github.com/NilFoundation/nil/nil/internal/telemetry"
	"github.com/NilFoundation/nil/nil/internal/types"
)

type RunMode int

const (
	NormalRunMode RunMode = iota
	CollatorsOnlyRunMode
	BlockReplayRunMode
)

type Config struct {
	// Set by the command line
	RunMode RunMode `yaml:"-"`

	// Shard configuration
	NShards        uint32            `yaml:"nShards"`
	RunOnlyShard   types.ShardId     `yaml:"runOnlyShard"`
	ShardEndpoints map[string]string `yaml:"shardEndpoints"`

	// RPC
	RPCPort int `yaml:"rpcPort"`

	// Admin
	AdminSocketPath string `yaml:"adminSocket"`

	// Keys
	MainKeysOutPath string `yaml:"mainKeysPath"`
	NetworkKeysPath string `yaml:"networkKeysPath"`

	GasPriceScale float64 `yaml:"gasPriceScale"`
	GasBasePrice  uint64  `yaml:"gasBasePrice"`

	// HttpUrl is calculated from RPCPort
	HttpUrl string `yaml:"-"`

	// Test-only
	GracefulShutdown     bool   `yaml:"-"`
	TraceEVM             bool   `yaml:"-"`
	CollatorTickPeriodMs uint32 `yaml:"-"`
	Topology             string `yaml:"-"`
	ZeroStateYaml        string `yaml:"-"`

	// Sub-configs
	Network   *network.Config            `yaml:"network"`
	Telemetry *telemetry.Config          `yaml:"telemetry"`
	ZeroState *execution.ZeroStateConfig `yaml:"zeroState"`
	Replay    *ReplayConfig              `yaml:"replay"`
}

func NewDefaultConfig() *Config {
	return &Config{
		RunMode: NormalRunMode,

		NShards:         5,
		RPCPort:         8529,
		MainKeysOutPath: "keys.yaml",
		NetworkKeysPath: "network-keys.yaml",

		GasPriceScale: 0.0,
		GasBasePrice:  10,

		GracefulShutdown: true,
		Topology:         collate.TrivialShardTopologyId,

		Network:   network.NewDefaultConfig(),
		Telemetry: telemetry.NewDefaultConfig(),
		Replay:    NewDefaultReplayConfig(),
	}
}

type ReplayConfig struct {
	BlockId types.BlockNumber `yaml:"blockId"`
	ShardId types.ShardId     `yaml:"shardId"`
}

func NewDefaultReplayConfig() *ReplayConfig {
	return &ReplayConfig{
		BlockId: 1,
		ShardId: 1,
	}
}

func (c *Config) IsShardActive(shardId types.ShardId) bool {
	if shardId == types.MainShardId { // Main shard is always active
		return true
	}
	return c.RunOnlyShard == 0 || c.RunOnlyShard == shardId
}
