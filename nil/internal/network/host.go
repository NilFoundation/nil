package network

import (
	"fmt"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/p2p/net/connmgr"
	"github.com/libp2p/go-libp2p/p2p/security/noise"
	quic "github.com/libp2p/go-libp2p/p2p/transport/quic"
	"github.com/libp2p/go-libp2p/p2p/transport/tcp"
)

type Host = host.Host

// newHost creates a new libp2p host. It must be closed after use.
func newHost(conf *Config) (Host, error) {
	connMgr, err := connmgr.NewConnManager(100, 400, connmgr.WithGracePeriod(time.Minute))
	if err != nil {
		return nil, err
	}

	addr := conf.IPV4Address
	if addr == "" {
		addr = "0.0.0.0"
	}

	options := []libp2p.Option{
		libp2p.Security(noise.ID, noise.New),
		libp2p.ConnectionManager(connMgr),
	}

	if conf.PrivateKey != nil {
		key, _, err := crypto.ECDSAKeyPairFromKey(conf.PrivateKey)
		if err != nil {
			return nil, err
		}
		options = append(options,
			libp2p.Identity(key))
	}

	if conf.TcpPort != 0 {
		options = append(options,
			libp2p.ListenAddrStrings(fmt.Sprintf("/ip4/%s/tcp/%d", addr, conf.TcpPort)),
			libp2p.Transport(tcp.NewTCPTransport),
		)
	}

	if conf.QuicPort != 0 {
		options = append(options,
			libp2p.ListenAddrStrings(fmt.Sprintf("/ip4/%s/udp/%d/quic", addr, conf.QuicPort)),
			libp2p.Transport(quic.NewTransport),
		)
	}

	return libp2p.New(options...)
}
