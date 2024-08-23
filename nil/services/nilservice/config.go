package nilservice

import (
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
	NShards              uint32
	RunOnlyShard         types.ShardId
	ShardEndpoints       map[string]string
	HttpUrl              string
	AdminSocketPath      string
	Topology             string
	ZeroStateYaml        string
	ZeroState            *execution.ZeroStateConfig
	MainKeysOutPath      string
	NetworkKeysPath      string
	CollatorTickPeriodMs uint32
	GracefulShutdown     bool
	TraceEVM             bool
	GasPriceScale        float64
	GasBasePrice         uint64
	RunMode              RunMode
	ReplayBlockId        types.BlockNumber
	ReplayShardId        types.ShardId

	Network   *network.Config
	Telemetry *telemetry.Config
}

func (c *Config) IsShardActive(shardId types.ShardId) bool {
	if shardId == types.MainShardId { // Main shard is always active
		return true
	}
	return c.RunOnlyShard == 0 || c.RunOnlyShard == shardId
}
