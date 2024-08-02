package network

import (
	"context"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/check"
	"github.com/NilFoundation/nil/common/logging"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
	"github.com/rs/zerolog"
)

type Manager struct {
	ctx context.Context

	host   Host
	pubSub *PubSub

	mdnsService mdns.Service

	logger zerolog.Logger
}

func NewManager(ctx context.Context, conf *Config) (*Manager, error) {
	check.PanicIfNot(conf.Enabled())

	logger := logging.NewLogger("network")

	h, err := newHost(conf, logger)
	if err != nil {
		return nil, err
	}

	logger.Info().Msgf("Identity: %s", h.ID())
	logger.Info().Msgf("Listening on addresses:\n%s", common.Join("\n", h.Addrs()...))

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

func (m *Manager) Connect(ctx context.Context, addr string) (PeerID, error) {
	m.logger.Debug().Msgf("Connecting to %s", addr)

	addrInfo, err := peer.AddrInfoFromString(addr)
	if err != nil {
		return "", err
	}
	if err := m.host.Connect(ctx, *addrInfo); err != nil {
		return "", err
	}
	return addrInfo.ID, nil
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
