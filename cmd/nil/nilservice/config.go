package nilservice

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
}
