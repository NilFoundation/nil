package connection_manager

import (
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
)

type PeerReputationTracker interface {
	ReportPeer(peer.ID, reputationChangeReason)
}

func TryGetPeerReputationTracker(host host.Host) PeerReputationTracker {
	notifee, ok := host.ConnManager().Notifee().(*notifiee)
	if !ok {
		return nil
	}
	return notifee
}
