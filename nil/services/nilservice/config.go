package nilservice

import (
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
	NShards              int
	RunOnlyShard         types.ShardId
	ShardEndpoints       map[string]string
	HttpPort             int
	AdminSocketPath      string
	Topology             string
	ZeroState            string
	MainKeysOutPath      string
	CollatorTickPeriodMs uint32
	GracefulShutdown     bool
	TraceEVM             bool
	GasPriceScale        float64
	GasBasePrice         uint64
	RunMode              RunMode
	ReplayBlockId        types.BlockNumber
	ReplayShardId        types.ShardId

	// network
	Libp2pTcpPort  int
	Libp2pQuicPort int
	UseMdns        bool

	Telemetry *telemetry.Config
}

func (c *Config) IsShardActive(shardId types.ShardId) bool {
	if shardId == types.MainShardId { // Main shard is always active
		return true
	}
	return c.RunOnlyShard == 0 || c.RunOnlyShard == shardId
}
