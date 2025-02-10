package nilservice

import (
	"crypto/ecdsa"
	"errors"
	"fmt"
	"slices"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/collate"
	"github.com/NilFoundation/nil/nil/internal/config"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/keys"
	"github.com/NilFoundation/nil/nil/internal/network"
	"github.com/NilFoundation/nil/nil/internal/telemetry"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/cometa"
	"github.com/ethereum/go-ethereum/crypto"
)

var Logger = logging.NewLogger("config")

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

	// Admin
	AdminSocketPath string `yaml:"adminSocket,omitempty"`

	// Keys
	MainKeysPath         string                     `yaml:"mainKeysPath,omitempty"`
	NetworkKeysPath      string                     `yaml:"networkKeysPath,omitempty"`
	ValidatorKeysPath    string                     `yaml:"validatorKeysPath,omitempty"`
	ValidatorKeysManager *keys.ValidatorKeysManager `yaml:"-"`

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

	// Consensus
	Validators map[types.ShardId][]config.ValidatorInfo `yaml:"validators,omitempty"`

	// Sub-configs
	Network   *network.Config            `yaml:"network,omitempty"`
	Telemetry *telemetry.Config          `yaml:"telemetry,omitempty"`
	ZeroState *execution.ZeroStateConfig `yaml:"zeroState,omitempty"`
	Replay    *ReplayConfig              `yaml:"replay,omitempty"`
	Cometa    *cometa.Config             `yaml:"cometa,omitempty"`
	RpcNode   *RpcNodeConfig             `yaml:"rpcNode,omitempty"`
}

const DefaultNShards types.ShardId = 5

func NewDefaultConfig() *Config {
	return &Config{
		RunMode: NormalRunMode,

		NShards:           uint32(DefaultNShards),
		MainKeysPath:      "keys.yaml",
		NetworkKeysPath:   "network-keys.yaml",
		ValidatorKeysPath: "validator-keys.yaml",
		GasPriceScale:     0.0,
		GasBasePrice:      10,

		GracefulShutdown: true,
		Topology:         collate.TrivialShardTopologyId,

		Validators: make(map[types.ShardId][]config.ValidatorInfo),

		Network:   network.NewDefaultConfig(),
		Telemetry: telemetry.NewDefaultConfig(),
		Replay:    NewDefaultReplayConfig(),
		RpcNode:   NewDefaultRpcNodeConfig(),
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
	return c.RunMode == NormalRunMode && !c.SplitShards
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
			return fmt.Errorf("Shard %d is out of range (nShards = %d)", shard, c.NShards)
		}
	}

	if c.GasPriceScale < 0 {
		return errors.New("GasPriceScale must be >= 0")
	}

	return nil
}

func (c *Config) loadValidatorKeys() error {
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

func (c *Config) LoadValidatorPrivateKey() (key *ecdsa.PrivateKey, err error) {
	defer func() {
		if err == nil {
			Logger.Trace().
				Hex(logging.FieldPublicKey, crypto.FromECDSAPub(&key.PublicKey)).
				Msg("Loaded validator key")
		}
	}()
	if err := c.loadValidatorKeys(); err != nil {
		return nil, err
	}
	return c.ValidatorKeysManager.GetKey()
}

func (c *Config) BlockGeneratorParams(shardId types.ShardId) execution.BlockGeneratorParams {
	return execution.BlockGeneratorParams{
		ShardId:       shardId,
		NShards:       c.NShards,
		TraceEVM:      c.TraceEVM,
		Timer:         common.NewTimer(),
		GasBasePrice:  types.NewValueFromUint64(c.GasBasePrice),
		GasPriceScale: c.GasPriceScale,
	}
}
