package network

import (
	"crypto/ecdsa"

	"github.com/libp2p/go-libp2p/core/peer"
)

type PeerID = peer.ID

type Config struct {
	PrivateKey *ecdsa.PrivateKey
	TcpPort    int
	QuicPort   int
	UseMdns    bool
}

func (c *Config) Enabled() bool {
	return c.TcpPort != 0 || c.QuicPort != 0
}
