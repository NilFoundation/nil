package network

import cm "github.com/NilFoundation/nil/nil/internal/network/connection_manager"

func TryGetPeerReputationTracker(manager *Manager) cm.PeerReputationTracker {
	return cm.TryGetPeerReputationTracker(manager.host)
}
