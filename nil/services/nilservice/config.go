package nilservice

import (
	"slices"

	"github.com/NilFoundation/nil/nil/common/check"
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
	ArchiveRunMode
	RpcRunMode
)

type Config struct {
	// Set by the command line
	RunMode RunMode `yaml:"-"`

	// Shard configuration
	NShards     uint32 `yaml:"nShards,omitempty"`
	MyShards    []uint `yaml:"myShards,omitempty"`
	SplitShards bool   `yaml:"splitShards,omitempty"`

	// RPC
	RPCPort       int    `yaml:"rpcPort,omitempty"`
	BootstrapPeer string `yaml:"bootstrapPeer,omitempty"`

	// Admin
	AdminSocketPath string `yaml:"adminSocket,omitempty"`

	// Keys
	MainKeysOutPath string `yaml:"mainKeysPath,omitempty"`
	NetworkKeysPath string `yaml:"networkKeysPath,omitempty"`

	GasPriceScale float64 `yaml:"gasPriceScale,omitempty"`
	GasBasePrice  uint64  `yaml:"gasBasePrice,omitempty"`

	// HttpUrl is calculated from RPCPort
	HttpUrl string `yaml:"-"`

	// Test-only
	GracefulShutdown     bool   `yaml:"-"`
	TraceEVM             bool   `yaml:"-"`
	CollatorTickPeriodMs uint32 `yaml:"-"`
	Topology             string `yaml:"-"`
	ZeroStateYaml        string `yaml:"-"`

	// Sub-configs
	Network   *network.Config            `yaml:"network,omitempty"`
	Telemetry *telemetry.Config          `yaml:"telemetry,omitempty"`
	ZeroState *execution.ZeroStateConfig `yaml:"zeroState,omitempty"`
	Replay    *ReplayConfig              `yaml:"replay,omitempty"`
}

func NewDefaultConfig() *Config {
	return &Config{
		RunMode: NormalRunMode,

		MyShards:        []uint{},
		NShards:         5,
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
	BlockIdFirst types.BlockNumber `yaml:"blockIdFirst"`
	BlockIdLast  types.BlockNumber `yaml:"blockIdLast"`
	ShardId      types.ShardId     `yaml:"shardId"`
}

func NewDefaultReplayConfig() *ReplayConfig {
	return &ReplayConfig{
		BlockIdFirst: 1,
		ShardId:      1,
	}
}

func (c *Config) GetMyShards() []uint {
	if !c.SplitShards {
		shards := make([]uint, c.NShards)
		for i := range shards {
			shards[i] = uint(i)
		}
		return shards
	}

	return c.MyShards
}

func (c *Config) IsShardActive(shardId types.ShardId) bool {
	if !c.SplitShards {
		check.PanicIfNotf(shardId < types.ShardId(c.NShards), "shardId %d is out of range", shardId)
		return true
	}
	return slices.Contains(c.MyShards, uint(shardId))
}
