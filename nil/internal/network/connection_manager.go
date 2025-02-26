package network

import (
	"context"
	"sync"
	"time"

	libp2pconnmgr "github.com/libp2p/go-libp2p/core/connmgr"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/net/connmgr"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/rs/zerolog"
)

type WithCustomNotifeeDecorator struct {
	libp2pconnmgr.ConnManager

	notifee network.Notifiee
}

func (cm *WithCustomNotifeeDecorator) Notifee() network.Notifiee {
	return cm.notifee
}

var _ libp2pconnmgr.ConnManager = (*WithCustomNotifeeDecorator)(nil)

type PeerReputationTracker interface {
	ReportPeer(peer.ID)
}

type peerInfo struct {
	lastBannedTime time.Time
}

type notifiee struct {
	basicNotifee network.Notifiee

	peerBanTimeout time.Duration // +checklocksignore: constant

	peerReputations map[peer.ID]peerInfo // +checklocks:mu
	mu              sync.Mutex

	logger zerolog.Logger // +checklocksignore: thread safe
}

func (n *notifiee) Listen(network network.Network, address ma.Multiaddr) {
	n.basicNotifee.Listen(network, address)
}

func (n *notifiee) ListenClose(network network.Network, address ma.Multiaddr) {
	n.basicNotifee.ListenClose(network, address)
}

func (n *notifiee) Connected(network network.Network, connection network.Conn) {
	n.basicNotifee.Connected(network, connection)

	peer := connection.RemotePeer()

	n.mu.Lock()
	defer n.mu.Unlock()

	if n.isBanned(peer) {
		if err := network.ClosePeer(peer); err != nil {
			n.logger.Error().Err(err).Msgf("Failed to close peer %s", peer)
		}
	}
}

func (n *notifiee) Disconnected(network network.Network, connection network.Conn) {
	n.basicNotifee.Disconnected(network, connection)
}

// +checklocks:n.mu
func (n *notifiee) isBanned(peer peer.ID) bool {
	if reputation, ok := n.peerReputations[peer]; ok {
		return time.Since(reputation.lastBannedTime) < n.peerBanTimeout
	}
	return false
}

func (n *notifiee) ReportPeer(peer peer.ID) {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.recalculateReputation()

	n.peerReputations[peer] = peerInfo{
		lastBannedTime: time.Now(),
	}

	// TODO: drop connections
}

// +checklocks:n.mu
func (n *notifiee) recalculateReputation() {
	for peer := range n.peerReputations {
		if !n.isBanned(peer) {
			delete(n.peerReputations, peer)
		}
	}
}

func (n *notifiee) start(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				func() {
					n.mu.Lock()
					defer n.mu.Unlock()

					n.recalculateReputation()
				}()
			}
		}
	}()
}

var _ network.Notifiee = (*notifiee)(nil)

func newConnectionManagerWithPeerReputationTracking(
	ctx context.Context,
	conf *Config,
	logger zerolog.Logger,
	low, hi int,
	opts ...connmgr.Option,
) (libp2pconnmgr.ConnManager, error) {
	baseConnectionManager, err := connmgr.NewConnManager(low, hi, opts...)
	if err != nil {
		return nil, err
	}
	notifee := &notifiee{
		basicNotifee:    baseConnectionManager.Notifee(),
		peerBanTimeout:  conf.ConnectionManagerConfig.PeerBanTimeout,
		peerReputations: make(map[peer.ID]peerInfo),
		logger:          logger,
	}
	notifee.start(ctx)
	return &WithCustomNotifeeDecorator{
		ConnManager: baseConnectionManager,
		notifee:     notifee,
	}, nil
}

func TryGetPeerReputationTracker(host host.Host) PeerReputationTracker {
	notifee, ok := host.ConnManager().Notifee().(*notifiee)
	if !ok {
		return nil
	}
	return notifee
}
