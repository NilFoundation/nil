package network

import (
	"fmt"
	"time"

	"github.com/NilFoundation/nil/common/logging"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/p2p/net/connmgr"
	"github.com/libp2p/go-libp2p/p2p/security/noise"
	quic "github.com/libp2p/go-libp2p/p2p/transport/quic"
	"github.com/libp2p/go-libp2p/p2p/transport/tcp"
	"github.com/rs/zerolog"
)

type Host = host.Host

// newHost creates a new libp2p host. It must be closed after use.
func newHost(conf *Config, logger zerolog.Logger) (Host, error) {
	connMgr, err := connmgr.NewConnManager(100, 400, connmgr.WithGracePeriod(time.Minute))
	if err != nil {
		return nil, err
	}

	options := []libp2p.Option{
		libp2p.Identity(conf.PrivateKey),
		libp2p.Security(noise.ID, noise.New),
		libp2p.ConnectionManager(connMgr),
	}

	if conf.TcpPort != 0 {
		options = append(options,
			libp2p.ListenAddrStrings(fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", conf.TcpPort)),
			libp2p.Transport(tcp.NewTCPTransport),
		)

		logger.Info().
			Int(logging.FieldTcpPort, conf.TcpPort).
			Msg("Listening on TCP...")
	}

	if conf.QuicPort != 0 {
		options = append(options,
			libp2p.ListenAddrStrings(fmt.Sprintf("/ip4/0.0.0.0/udp/%d/quic", conf.QuicPort)),
			libp2p.Transport(quic.NewTransport),
		)

		logger.Info().
			Int(logging.FieldQuicPort, conf.QuicPort).
			Msg("Listening on QUIC...")
	}

	return libp2p.New(options...)
}
