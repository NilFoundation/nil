package network

import (
	"context"

	"github.com/NilFoundation/nil/common/check"
	"github.com/NilFoundation/nil/common/logging"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
	"github.com/rs/zerolog"
)

type Config struct {
	PrivateKey crypto.PrivKey
	TcpPort    int
	QuicPort   int
	UseMdns    bool
}

type Manager struct {
	ctx context.Context

	host   Host
	pubSub *PubSub

	mdnsService mdns.Service

	logger zerolog.Logger
}

func (c *Config) Enabled() bool {
	return c.TcpPort != 0 || c.QuicPort != 0
}

func GeneratePrivateKey() crypto.PrivKey {
	res, _, err := crypto.GenerateKeyPair(crypto.Ed25519, -1)
	check.PanicIfErr(err)
	return res
}

func NewManager(ctx context.Context, conf *Config) (*Manager, error) {
	check.PanicIfNot(conf.Enabled())

	logger := logging.NewLogger("network")

	h, err := newHost(conf, logger)
	if err != nil {
		return nil, err
	}

	ps, err := newPubSub(ctx, h)
	if err != nil {
		return nil, err
	}

	var ms mdns.Service
	if conf.UseMdns {
		ms, err = setupMdnsDiscovery(ctx, h)
		if err != nil {
			return nil, err
		}
	}

	return &Manager{
		ctx:         ctx,
		host:        h,
		pubSub:      ps,
		mdnsService: ms,
		logger:      logger,
	}, nil
}

func (m *Manager) PubSub() *PubSub {
	return m.pubSub
}

func (m *Manager) Close() {
	if m.mdnsService != nil {
		if err := m.mdnsService.Close(); err != nil {
			m.logError(err, "Error closing mDNS service")
		}
	}

	if err := m.pubSub.Close(); err != nil {
		m.logError(err, "Error closing pubsub")
	}

	if err := m.host.Close(); err != nil {
		m.logError(err, "Error closing host")
	}
}

func (m *Manager) logError(err error, msg string) {
	if m.ctx.Err() != nil {
		// If we're already closing, no need to log errors.
		return
	}

	m.logger.Error().Err(err).Msg(msg)
}
