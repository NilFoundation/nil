package nilservice

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/collate"
	"github.com/NilFoundation/nil/nil/internal/config"
	"github.com/NilFoundation/nil/nil/internal/crypto/bls"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/keys"
	"github.com/NilFoundation/nil/nil/internal/network"
	"github.com/NilFoundation/nil/nil/internal/telemetry"
	"github.com/NilFoundation/nil/nil/internal/tracing"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/cometa"
	"github.com/NilFoundation/nil/nil/services/indexer"
	"github.com/NilFoundation/nil/nil/services/rollup"
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
	RPCPort        int                   `yaml:"rpcPort,omitempty"`
	BootstrapPeers network.AddrInfoSlice `yaml:"bootstrapPeers,omitempty"`
	EnableDevApi   bool                  `yaml:"enableDevApi,omitempty"`

	// Profiling
	PprofPort int `yaml:"pprofPort,omitempty"`

	// Admin
	AdminSocketPath string `yaml:"adminSocket,omitempty"`
	AllowDbDrop     bool   `yaml:"allowDbDrop,omitempty"`

	// RPC events log
	LogClientRpcEvents bool `yaml:"logClientRpcEvents,omitempty"`

	// Keys
	MainKeysPath         string                     `yaml:"mainKeysPath,omitempty"`
	ValidatorKeysPath    string                     `yaml:"validatorKeysPath,omitempty"`
	ValidatorKeysManager *keys.ValidatorKeysManager `yaml:"-"`

	// HttpUrl is calculated from RPCPort
	HttpUrl string `yaml:"-"`

	// Test-only
	GracefulShutdown     bool   `yaml:"-"`
	TraceEVM             bool   `yaml:"-"`
	CollatorTickPeriodMs uint32 `yaml:"-"`
	SyncTimeoutFactor    uint32 `yaml:"-"`
	Topology             string `yaml:"-"`
	EnableConfigCache    bool   `yaml:"-"`

	// Consensus
	Validators       map[types.ShardId][]config.ValidatorInfo `yaml:"validators,omitempty"`
	DisableConsensus bool                                     `yaml:"-"`

	// Sub-configs
	Network   *network.Config            `yaml:"network,omitempty"`
	Telemetry *telemetry.Config          `yaml:"telemetry,omitempty"`
	ZeroState *execution.ZeroStateConfig `yaml:"zeroState,omitempty"`
	Replay    *ReplayConfig              `yaml:"replay,omitempty"`
	Cometa    *cometa.Config             `yaml:"cometa,omitempty"`
	Indexer   *indexer.Config            `yaml:"indexer,omitempty"`
	RpcNode   *RpcNodeConfig             `yaml:"rpcNode,omitempty"`

	L1Fetcher rollup.L1BlockFetcher `yaml:"-"`

	FeeCalculator         execution.FeeCalculator                                                   `yaml:"-"`
	NetworkManagerFactory func(ctx context.Context, cfg *Config, db db.DB) (network.Manager, error) `yaml:"-"`
}

const (
	DefaultNShards   types.ShardId = 5
	DefaultPprofPort uint32        = 6060
)

func NewDefaultConfig() *Config {
	return &Config{
		RunMode: NormalRunMode,

		NShards:           uint32(DefaultNShards),
		MainKeysPath:      "keys.yaml",
		ValidatorKeysPath: "validator-keys.yaml",

		GracefulShutdown:  true,
		Topology:          collate.TrivialShardTopologyId,
		EnableConfigCache: true,

		Validators: make(map[types.ShardId][]config.ValidatorInfo),

		Network:   network.NewDefaultConfig(),
		Telemetry: telemetry.NewDefaultConfig(),
		Replay:    NewDefaultReplayConfig(),
		RpcNode:   NewDefaultRpcNodeConfig(),
		PprofPort: int(DefaultPprofPort),
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

type RpcNodeConfig struct {
	ArchiveNodeList network.AddrInfoSlice `yaml:"archiveNodeList,omitempty"`
}

func NewDefaultRpcNodeConfig() *RpcNodeConfig {
	return &RpcNodeConfig{}
}

func (c *Config) GetMyShards() []uint {
	shards := c.MyShards
	if len(shards) > 0 {
		return shards
	}

	if !c.SplitShards {
		shards = make([]uint, c.NShards)
		for i := range shards {
			shards[i] = uint(i)
		}
	}

	return shards
}

func (c *Config) IsShardActive(shardId types.ShardId) bool {
	if !c.SplitShards {
		return true
	}
	return slices.Contains(c.MyShards, uint(shardId))
}

func (c *Config) IsFaucetApiEnabled() bool {
	return (c.RunMode == NormalRunMode || c.RunMode == RpcRunMode) && !c.SplitShards
}

func (c *Config) ShouldStartCometa() bool {
	return c.RunMode == NormalRunMode && !c.SplitShards
}

func (c *Config) Validate() error {
	if c.NShards < 2 {
		return errors.New("NShards must be greater than 2 (main shard + 1)")
	}

	for _, shard := range c.MyShards {
		if shard >= uint(c.NShards) {
			return fmt.Errorf("shard %d is out of range (nShards = %d)", shard, c.NShards)
		}
	}

	if c.MyShards != nil && !c.DisableConsensus {
		if !slices.Contains(c.MyShards, uint(types.MainShardId)) {
			return errors.New("main shard must be included in MyShards")
		}
		if len(c.MyShards) == 1 {
			return errors.New("standalone main shard doesn't make sense with enabled consensus")
		}
	}

	return nil
}

func (c *Config) LoadValidatorKeys() error {
	if c.ValidatorKeysPath == "" {
		return nil
	}

	if c.ValidatorKeysManager == nil {
		c.ValidatorKeysManager = keys.NewValidatorKeyManager(c.ValidatorKeysPath)
		if err := c.ValidatorKeysManager.InitKey(); err != nil {
			return err
		}
	}

	return nil
}

func (c *Config) LoadValidatorPrivateKey() (bls.PrivateKey, error) {
	if err := c.LoadValidatorKeys(); err != nil {
		return nil, err
	}
	return c.ValidatorKeysManager.GetKey()
}

func (c *Config) BlockGeneratorParams(shardId types.ShardId) execution.BlockGeneratorParams {
	var verboseTracingHook *tracing.Hooks
	if c.TraceEVM {
		verboseTracingHook = execution.VerboseTracingHooks(logging.NewLogger("tracer"))
	}
	return execution.BlockGeneratorParams{
		ShardId:          shardId,
		NShards:          c.NShards,
		EvmTracingHooks:  verboseTracingHook,
		MainKeysPath:     c.MainKeysPath,
		DisableConsensus: c.DisableConsensus,
		FeeCalculator:    c.FeeCalculator,
	}
}
