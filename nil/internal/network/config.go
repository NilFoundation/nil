package network

import (
	"github.com/libp2p/go-libp2p/core/peer"
)

type PeerID = peer.ID

type Config struct {
	PrivateKey PrivateKey `yaml:"-"`

	IPV4Address string `yaml:"-"`
	TcpPort     int    `yaml:"tcpPort"`
	QuicPort    int    `yaml:"quicPort"`

	UseMdns bool `yaml:"useMdns"`

	DHTEnabled        bool     `yaml:"dhtEnabled"`
	DHTBootstrapPeers []string `yaml:"dhtBootstrapPeers"`
}

func NewDefaultConfig() *Config {
	return &Config{}
}

func (c *Config) Enabled() bool {
	return c.TcpPort != 0 || c.QuicPort != 0
}
