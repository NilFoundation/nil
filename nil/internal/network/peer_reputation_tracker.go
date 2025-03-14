package network

import "github.com/NilFoundation/nil/nil/internal/network/connection_manager"

func TryGetPeerReputationTracker(manager *Manager) connection_manager.PeerReputationTracker {
	return connection_manager.TryGetPeerReputationTracker(manager.host)
}
