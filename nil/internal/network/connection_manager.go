package network

import (
	"context"
	"math"
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
	reputation     Reputation
}

// TODO: move to ConnectionManagerConfig
type ReputationConfig struct {
	decayDevider           int
	reputationBanThreshold Reputation
}

type notifiee struct {
	basicNotifee network.Notifiee

	decayDevider     int
	peerBanTimeout   time.Duration // +checklocksignore: constant
	reputationConfig ReputationConfig

	peerReputations  map[peer.ID]*peerInfo // +checklocks:mu
	lastUpdateSecond int64                 // +checklocks:mu
	mu               sync.Mutex

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

	n.recalculateReputationsAccordingToCurrentTime()

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
	if peerInfo, ok := n.peerReputations[peer]; ok {
		return peerInfo.reputation < n.reputationConfig.reputationBanThreshold
	}
	return false
}

func (n *notifiee) ReportPeer(peer peer.ID) {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.recalculateReputationsAccordingToCurrentTime()

	n.peerReputations[peer] = &peerInfo{
		lastBannedTime: time.Now(),
	}

	// TODO: drop connections
}

// Reputation represents reputation value of the node
type Reputation int32

// add handles overflow and underflow condition while adding two Reputation values.
func (r Reputation) add(num Reputation) Reputation {
	if num > 0 {
		if r > math.MaxInt32-num {
			return math.MaxInt32
		}
	} else if r < math.MinInt32-num {
		return math.MinInt32
	}
	return r + num
}

// sub handles underflow condition while subtracting two Reputation values.
func (r Reputation) sub(num Reputation) Reputation {
	if num < 0 {
		if r > math.MaxInt32+num {
			return math.MaxInt32
		}
	} else if r < math.MinInt32+num {
		return math.MinInt32
	}
	return r - num
}

// calculateDivider calculates the divider for the reputation decay formula.
// t is the time in seconds for the reputation to decay to p.
func calculateDecayDivider(t int, p float64) int {
	fd := 1.0 / (1.0 - math.Pow(p, 1.0/float64(t)))
	di := int(math.Floor(fd))
	if di < 1 {
		di = 1
	}
	return di
}

// TODO: comment
func (n *notifiee) reputationTick(reput Reputation) Reputation {
	diff := Reputation(int(reput) / n.decayDevider)
	if diff == 0 && reput < 0 {
		diff = -1
	} else if diff == 0 && reput > 0 {
		diff = 1
	}
	return reput.sub(diff)
}

// +checklocks:n.mu
func (n *notifiee) recalculateReputationsAccordingToCurrentTime() {
	n.mu.Lock()
	defer n.mu.Unlock()

	currentSecond := time.Now().Unix() // TODO: use mockable time
	elapsedSeconds := currentSecond - n.lastUpdateSecond
	n.lastUpdateSecond = currentSecond

	for _ = range elapsedSeconds {
		for _, info := range n.peerReputations {
			info.reputation = n.reputationTick(info.reputation)
		}
	}
	// TODO: add forget mechanism
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

					n.recalculateReputationsAccordingToCurrentTime()
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
		basicNotifee: baseConnectionManager.Notifee(),
		reputationConfig: ReputationConfig{
			// A bit low, then 35 seconds to reduce reputation by half. Or about 2% per second.
			decayDevider:           calculateDecayDivider(35, 0.5),
			reputationBanThreshold: -50,
		},
		peerBanTimeout:  conf.ConnectionManagerConfig.PeerBanTimeout,
		peerReputations: make(map[peer.ID]*peerInfo),
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
