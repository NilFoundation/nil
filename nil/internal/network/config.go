package network

import (
	"github.com/libp2p/go-libp2p/core/peer"
)

type PeerID = peer.ID

type Config struct {
	PrivateKey PrivateKey `yaml:"-"`

	IPV4Address string `yaml:"ipv4,omitempty"`
	TcpPort     int    `yaml:"tcpPort,omitempty"`
	QuicPort    int    `yaml:"quicPort,omitempty"`

	DHTEnabled        bool     `yaml:"dhtEnabled,omitempty"`
	DHTBootstrapPeers []string `yaml:"dhtBootstrapPeers,omitempty"`
}

func NewDefaultConfig() *Config {
	return &Config{}
}

func (c *Config) Enabled() bool {
	return c.TcpPort != 0 || c.QuicPort != 0
}
