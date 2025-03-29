package network

import (
	"testing"

	"github.com/libp2p/go-libp2p/core/peer"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestConfigYamlSerialization(t *testing.T) {
	t.Parallel()

	addrInfo1 := AddrInfo{}
	err := addrInfo1.Set("/ip4/127.0.0.1/tcp/1500/p2p/16Uiu2HAmQctkUi9y7WfUtYa9rPon1m5TRBtXSvUwi2VtpbWZj4yf")
	require.NoError(t, err)

	addrInfo2 := AddrInfo{}
	err = addrInfo2.Set("/ip4/192.168.1.1/tcp/4002/p2p/16Uiu2HAmQctkUi9y7WfUtYa9rPon1m5TRBtXSvUwi2VtpbWZj4yg")
	require.NoError(t, err)

	config := Config{
		DHTBootstrapPeers: AddrInfoSlice{
			addrInfo1,
			addrInfo2,
		},
	}

	data, err := yaml.Marshal(config)
	require.NoError(t, err)

	expectedYaml := `dhtBootstrapPeers:
- /ip4/127.0.0.1/tcp/1500/p2p/16Uiu2HAmQctkUi9y7WfUtYa9rPon1m5TRBtXSvUwi2VtpbWZj4yf
- /ip4/192.168.1.1/tcp/4002/p2p/16Uiu2HAmQctkUi9y7WfUtYa9rPon1m5TRBtXSvUwi2VtpbWZj4yg
`
	require.YAMLEq(t, expectedYaml, string(data))

	var deserialized Config
	err = yaml.Unmarshal(data, &deserialized)
	require.NoError(t, err)

	require.Equal(t, config, deserialized)
}

func TestAddrInfoStringRepresentation(t *testing.T) {
	t.Parallel()

	addr1, err := ma.NewMultiaddr("/ip4/127.0.0.1/tcp/1500/p2p/16Uiu2HAmQctkUi9y7WfUtYa9rPon1m5TRBtXSvUwi2VtpbWZj4yf")
	require.NoError(t, err)

	addr2, err := ma.NewMultiaddr(
		"/ip4/192.168.0.10/tcp/4002/p2p/16Uiu2HAmQctkUi9y7WfUtYa9rPon1m5TRBtXSvUwi2VtpbWZj4yf")
	require.NoError(t, err)

	_, id := peer.SplitAddr(addr1)
	addrInfo := AddrInfo{
		ID:    id,
		Addrs: []ma.Multiaddr{addr1, addr2},
	}

	expectedString := "/ip4/127.0.0.1/tcp/1500/p2p/16Uiu2HAmQctkUi9y7WfUtYa9rPon1m5TRBtXSvUwi2VtpbWZj4yf" +
		"/p2p/16Uiu2HAmQctkUi9y7WfUtYa9rPon1m5TRBtXSvUwi2VtpbWZj4yf," +
		"/ip4/192.168.0.10/tcp/4002/p2p/16Uiu2HAmQctkUi9y7WfUtYa9rPon1m5TRBtXSvUwi2VtpbWZj4yf" +
		"/p2p/16Uiu2HAmQctkUi9y7WfUtYa9rPon1m5TRBtXSvUwi2VtpbWZj4yf"
	require.Equal(t, expectedString, addrInfo.String())
}
