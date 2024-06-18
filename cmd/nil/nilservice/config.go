package nilservice

type Config struct {
	NShards         int
	HttpPort        int
	Topology        string
	ZeroState       string
	MainKeysOutPath string
}
