package nilservice

import (
	"github.com/NilFoundation/nil/core/types"
)

type RunMode int

const (
	NormalRunMode RunMode = iota
	CollatorsOnlyRunMode
	BlockReplayRunMode
)

type Config struct {
	NShards              int
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
}
