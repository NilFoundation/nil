//go:build test

package network

import (
	"context"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"
)

func CalcAddress(m *Manager) AddrInfo {
	addr, err := peer.AddrInfoFromString(m.host.Addrs()[0].String() + "/p2p/" + m.host.ID().String())
	check.PanicIfErr(err)
	return AddrInfo(*addr)
}

func NewTestManagerWithBaseConfig(t *testing.T, ctx context.Context, conf *Config) *Manager {
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

	m, err := NewManager(ctx, conf)
	require.NoError(t, err)
	return m
}

func NewTestManager(t *testing.T, ctx context.Context) *Manager {
	t.Helper()

	return NewTestManagerWithBaseConfig(t, ctx, &Config{})
}

func NewTestManagers(t *testing.T, ctx context.Context, initialTcpPort int, n int) []*Manager {
	t.Helper()

	managers := make([]*Manager, n)
	cfg := &Config{}
	for i := range n {
		cfg.TcpPort = initialTcpPort + i
		managers[i] = NewTestManagerWithBaseConfig(t, ctx, cfg)
	}
	return managers
}

func ConnectManagers(t *testing.T, m1, m2 *Manager) (PeerID, PeerID) {
	t.Helper()

	id, err := m1.Connect(m1.ctx, CalcAddress(m2))
	require.NoError(t, err)
	require.Equal(t, m2.host.ID(), id)

	WaitForPeer(t, m2, m1.host.ID())
	return m1.host.ID(), m2.host.ID()
}

func ConnectAllManagers(t *testing.T, managers ...*Manager) {
	t.Helper()

	for i := range len(managers) - 1 {
		for j := i + 1; j < len(managers); j++ {
			ConnectManagers(t, managers[i], managers[j])
		}
	}
}

func WaitForPeer(t *testing.T, m *Manager, id PeerID) {
	t.Helper()

	require.Eventually(t, func() bool {
		return slices.Contains(m.host.Peerstore().Peers(), id)
	}, 10*time.Second, 100*time.Millisecond)
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
