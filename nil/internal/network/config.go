package network

import (
	"time"

	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/peer"
)

type PeerID = peer.ID

type ConnectionManagerConfig struct {
	PeerBanTimeout time.Duration `yaml:"peerBanTimeout,omitempty"`
}

type Config struct {
	PrivateKey PrivateKey `yaml:"-"`

	Prefix      string `yaml:"prefix,omitempty"`
	IPV4Address string `yaml:"ipv4,omitempty"`
	TcpPort     int    `yaml:"tcpPort,omitempty"`
	QuicPort    int    `yaml:"quicPort,omitempty"`

	DHTEnabled        bool          `yaml:"dhtEnabled,omitempty"`
	DHTBootstrapPeers AddrInfoSlice `yaml:"dhtBootstrapPeers,omitempty"`
	DHTMode           dht.ModeOpt   `yaml:"-,omitempty"`

	ConnectionManagerConfig ConnectionManagerConfig `yaml:"connectionManager,omitempty"`
}

func NewDefaultConfig() *Config {
	return &Config{
		DHTMode: dht.ModeAutoServer,
		Prefix:  "/nil",
	}
}

func (c *Config) Enabled() bool {
	return c.TcpPort != 0 || c.QuicPort != 0
}
