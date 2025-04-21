package types

import (
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/network"
)

type BootstrapConfig struct {
	NShards           uint32                     `json:"nShards"`
	ZeroStateConfig   *execution.ZeroStateConfig `json:"zeroStateConfig,omitempty"`
	BootstrapPeers    network.AddrInfoSlice      `json:"bootstrapPeers,omitempty"`
	DhtBootstrapPeers network.AddrInfoSlice      `json:"bootstrapDhtPeers,omitempty"`
}
