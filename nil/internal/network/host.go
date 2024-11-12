package network

import (
	"context"
	"fmt"
	"time"

	"github.com/NilFoundation/nil/nil/internal/network/internal"
	"github.com/libp2p/go-libp2p"
	libp2pcrypto "github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/net/connmgr"
	"github.com/libp2p/go-libp2p/p2p/security/noise"
	quic "github.com/libp2p/go-libp2p/p2p/transport/quic"
	"github.com/libp2p/go-libp2p/p2p/transport/tcp"
)

type Host = host.Host

var defaultGracePeriod = connmgr.WithGracePeriod(time.Minute)

func getCommonOptions(ctx context.Context, privateKey libp2pcrypto.PrivKey) ([]libp2p.Option, error) {
	cm, err := connmgr.NewConnManager(100, 400, defaultGracePeriod)
	if err != nil {
		return nil, err
	}

	pid, err := peer.IDFromPublicKey(privateKey.GetPublic())
	if err != nil {
		return nil, err
	}

	metrics, err := internal.NewMetricsReporter(ctx, pid)
	if err != nil {
		return nil, err
	}

	return []libp2p.Option{
		libp2p.Security(noise.ID, noise.New),
		libp2p.ConnectionManager(cm),
		libp2p.Identity(privateKey),
		libp2p.BandwidthReporter(metrics),
	}, nil
}

// newHost creates a new libp2p host. It must be closed after use.
func newHost(ctx context.Context, conf *Config) (Host, error) {
	addr := conf.IPV4Address
	if addr == "" {
		addr = "0.0.0.0"
	}

	options, err := getCommonOptions(ctx, conf.PrivateKey)
	if err != nil {
		return nil, err
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

// newClient creates a new libp2p host that doesn't listen to any port. It must be closed after use.
func newClient(ctx context.Context, conf *Config) (Host, error) {
	var privateKey libp2pcrypto.PrivKey
	if conf != nil && conf.PrivateKey != nil {
		privateKey = conf.PrivateKey
	}
	if privateKey == nil {
		var err error
		privateKey, err = GeneratePrivateKey()
		if err != nil {
			return nil, err
		}
	}

	options, err := getCommonOptions(ctx, privateKey)
	if err != nil {
		return nil, err
	}
	options = append(options, libp2p.NoListenAddrs)
	return libp2p.New(options...)
}
