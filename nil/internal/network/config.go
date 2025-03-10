package network

import (
	"github.com/NilFoundation/nil/nil/common/check"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type PeerID = peer.ID

type Config struct {
	PrivateKey PrivateKey `yaml:"-"`

	KeysPath string `yaml:"keysPath,omitempty"`

	Prefix      string `yaml:"prefix,omitempty"`
	IPV4Address string `yaml:"ipv4,omitempty"`
	TcpPort     int    `yaml:"tcpPort,omitempty"`
	QuicPort    int    `yaml:"quicPort,omitempty"`

	Relay bool `yaml:"relay,omitempty"`

	DHTEnabled        bool          `yaml:"dhtEnabled,omitempty"`
	DHTBootstrapPeers AddrInfoSlice `yaml:"dhtBootstrapPeers,omitempty"`
	DHTMode           dht.ModeOpt   `yaml:"-,omitempty"`
}

func NewDefaultConfig() *Config {
	return &Config{
		KeysPath: "network-keys.yaml",
		DHTMode:  dht.ModeAutoServer,
		Prefix:   "/nil",
	}
}

func (c *Config) Enabled() bool {
	return c.TcpPort != 0 || c.QuicPort != 0
}

func AddFlags(fset *pflag.FlagSet, cfg *Config) {
	fset.StringVar(&cfg.KeysPath, "keys-path", cfg.KeysPath, "path to libp2p keys")
	fset.IntVar(&cfg.TcpPort, "tcp-port", cfg.TcpPort, "tcp port for the network")
	fset.IntVar(&cfg.QuicPort, "quic-port", cfg.QuicPort, "quic port for the network")
	fset.BoolVar(&cfg.Relay, "relay", cfg.Relay, "enable relay")
	fset.BoolVar(&cfg.DHTEnabled, "with-discovery", cfg.DHTEnabled, "enable discovery (with Kademlia DHT)")
	fset.Var(&cfg.DHTBootstrapPeers, "discovery-bootstrap-peers", "bootstrap peers for discovery")
	check.PanicIfErr(fset.SetAnnotation("discovery-bootstrap-peers", cobra.BashCompOneRequiredFlag, []string{"with-discovery"}))
}
