//go:build test

package network

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"
)

func CalcAddress(m Manager) AddrInfo {
	addr, err := peer.AddrInfoFromString(hostAddress(m))
	check.PanicIfErr(err)
	return AddrInfo(*addr)
}

func hostAddress(m Manager) string {
	return m.getHost().Addrs()[0].String() + "/p2p/" + m.getHost().ID().String()
}

func NewTestManagerWithBaseConfig(ctx context.Context, t *testing.T, conf *Config) *BasicManager {
	t.Helper()

	conf = common.CopyPtr(conf)
	if conf.IPV4Address == "" {
		conf.IPV4Address = "127.0.0.1"
	}
	if conf.PrivateKey == nil {
		privateKey, err := GeneratePrivateKey()
		require.NoError(t, err)
		conf.PrivateKey = privateKey
	}

	m, err := NewManager(ctx, conf, nil)
	require.NoError(t, err)
	return m
}

func NewTestManagers(ctx context.Context, t *testing.T, initialTcpPort int, n int) []*BasicManager {
	t.Helper()

	managers := make([]*BasicManager, n)
	cfg := &Config{}
	for i := range n {
		cfg.TcpPort = initialTcpPort + i
		managers[i] = NewTestManagerWithBaseConfig(ctx, t, cfg)
	}
	return managers
}

func ConnectManagers(t *testing.T, m1, m2 Manager) (PeerID, PeerID) {
	t.Helper()

	id, err := m1.Connect(t.Context(), CalcAddress(m2))
	require.NoError(t, err)
	require.Equal(t, m2.getHost().ID(), id)

	WaitForPeer(t, m2, m1.getHost().ID())
	return m1.getHost().ID(), m2.getHost().ID()
}

func ConnectAllManagers(t *testing.T, managers ...*BasicManager) {
	t.Helper()

	for i := range len(managers) - 1 {
		for j := i + 1; j < len(managers); j++ {
			ConnectManagers(t, managers[i], managers[j])
		}
	}
}

func WaitForPeer(t *testing.T, m Manager, id PeerID) {
	t.Helper()

	require.Eventually(t, func() bool {
		return slices.Contains(m.getHost().Peerstore().Peers(), id)
	}, 10*time.Second, 100*time.Millisecond)
}

// topLevelTestName returns the top-level test name for a given test.
// It is used to generate unique (among parallel tests) prefixes for topic and protocol names.
func topLevelTestName(t *testing.T) string {
	t.Helper()

	return strings.Split(t.Name(), "/")[0]
}

func GenerateConfig(t *testing.T, port int) (*Config, AddrInfo) {
	t.Helper()

	key, err := GeneratePrivateKey()
	require.NoError(t, err)

	id, err := peer.IDFromPublicKey(key.GetPublic())
	require.NoError(t, err)

	address, err := peer.AddrInfoFromString(fmt.Sprintf("/ip4/127.0.0.1/tcp/%d/p2p/%s", port, id))
	require.NoError(t, err)

	return &Config{
		PrivateKey: key,
		TcpPort:    port,
		DHTEnabled: true,
		Prefix:     topLevelTestName(t),
	}, AddrInfo(*address)
}

func GenerateConfigs(t *testing.T, n uint32, port int) ([]*Config, AddrInfoSlice) {
	t.Helper()

	configs := make([]*Config, n)
	addresses := make(AddrInfoSlice, n)
	for i := range int(n) {
		configs[i], addresses[i] = GenerateConfig(t, port+i)
		configs[i].DHTBootstrapPeers = addresses
	}

	return configs, addresses
}
