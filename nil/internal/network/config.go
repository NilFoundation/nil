package network

import (
	cm "github.com/NilFoundation/nil/nil/internal/network/connection_manager"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
)

type PeerID = peer.ID

type Config struct {
	PrivateKey      PrivateKey `yaml:"-"`
	ProtocolVersion string     `yaml:"-"`

	KeysPath string `yaml:"keysPath,omitempty"`

	Prefix      string `yaml:"prefix,omitempty"`
	IPV4Address string `yaml:"ipv4,omitempty"`
	TcpPort     int    `yaml:"tcpPort,omitempty"`
	QuicPort    int    `yaml:"quicPort,omitempty"`

	ServeRelay bool          `yaml:"serveRelay,omitempty"`
	Relays     AddrInfoSlice `yaml:"relays,omitempty"`

	DHTEnabled        bool          `yaml:"dhtEnabled,omitempty"`
	DHTBootstrapPeers AddrInfoSlice `yaml:"dhtBootstrapPeers,omitempty"`
	DHTMode           dht.ModeOpt   `yaml:"-"`

	ConnectionManagerConfig *cm.Config `yaml:"connectionManager,omitempty"`

	// Test-only
	Reachability network.Reachability `yaml:"-"`
}

type Option func(cfg *Config) error

func NewDefaultConfig() *Config {
	return &Config{
		KeysPath:                "network-keys.yaml",
		DHTMode:                 dht.ModeAutoServer,
		Prefix:                  "/nil",
		ConnectionManagerConfig: cm.NewDefaultConfig(),
	}
}

func (c *Config) Enabled() bool {
	return c.TcpPort != 0 || c.QuicPort != 0
}

// Apply applies the given options to the config, returning the first error
// encountered (if any).
func (c *Config) Apply(opts ...Option) error {
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(c); err != nil {
			return err
		}
	}
	return nil
}
