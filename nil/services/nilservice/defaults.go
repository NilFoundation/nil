package nilservice

import (
	"fmt"

	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/internal/network"
)

const (
	ChainNameDevnet = "devnet"

	ChainNameTest = "test" // todo: remove after tests
)

type ChainDefaults struct {
	RelayAddresses     []string
	BootstrapAddresses []string

	Relays         network.AddrInfoSlice
	BootstrapPeers network.AddrInfoSlice
}

var (
	TestDefaults = newChainDefaults(
		[]string{
			"/ip4/100.100.35.125/tcp/3149/p2p/16Uiu2HAmFhv4FVhp3syxJ7uQCXm86Rbiu9Y6Cox7zBD757mBNNdK",
		},
		[]string{
			"/ip4/100.100.35.125/tcp/3149/p2p/16Uiu2HAmFhv4FVhp3syxJ7uQCXm86Rbiu9Y6Cox7zBD757mBNNdK/p2p-circuit/" +
				"p2p/16Uiu2HAmErDZcwxP6KQ6x1oPn8bY1Kkg9o39LvACcKjzJNTYpDNd",
		},
	)

	DevnetDefaults = newChainDefaults(
		nil,
		nil,
	)

	// DefaultsByChainName is a map of chain names to their default configurations (hard-coded).
	// Users may assume them to be valid.
	DefaultsByChainName = map[string]*ChainDefaults{
		ChainNameDevnet: DevnetDefaults,

		ChainNameTest: TestDefaults,
	}
)

func newChainDefaults(relayAddresses, bootstrapAddresses []string) *ChainDefaults {
	relays, err := network.AddrInfoSliceFromStrings(relayAddresses)
	check.PanicIfErr(err)
	bootstrapPeers, err := network.AddrInfoSliceFromStrings(bootstrapAddresses)
	check.PanicIfErr(err)

	return &ChainDefaults{
		RelayAddresses:     relayAddresses,
		BootstrapAddresses: bootstrapAddresses,

		Relays:         relays,
		BootstrapPeers: bootstrapPeers,
	}
}

func DefaultConfig(chainName string) (*Config, error) {
	if chainName == "" {
		return NewDefaultConfig(), nil
	}

	defaults, ok := DefaultsByChainName[chainName]
	if !ok {
		return nil, fmt.Errorf("unknown chain name: %s", chainName)
	}

	config := NewDefaultConfig()

	config.BootstrapPeers = defaults.BootstrapPeers

	config.Network.TcpPort = 3000
	config.Network.DHTEnabled = true
	config.Network.DHTBootstrapPeers = config.BootstrapPeers

	return config, nil
}
