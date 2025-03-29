package cmdflags

import (
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/internal/network"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func AddNetwork(fset *pflag.FlagSet, cfg *network.Config) {
	fset.StringVar(&cfg.KeysPath, "keys-path", cfg.KeysPath, "path to libp2p keys")

	fset.IntVar(&cfg.TcpPort, "tcp-port", cfg.TcpPort, "tcp port for the network")
	fset.IntVar(&cfg.QuicPort, "quic-port", cfg.QuicPort, "quic port for the network")

	fset.BoolVar(&cfg.ServeRelay, "serve-relay", cfg.ServeRelay, "enable relay")
	fset.Var(&cfg.Relays, "relays", "relay peers")

	fset.BoolVar(&cfg.DHTEnabled, "with-discovery", cfg.DHTEnabled, "enable discovery (with Kademlia DHT)")
	fset.Var(&cfg.DHTBootstrapPeers, "discovery-bootstrap-peers", "bootstrap peers for discovery")
	check.PanicIfErr(
		fset.SetAnnotation("discovery-bootstrap-peers", cobra.BashCompOneRequiredFlag, []string{"with-discovery"}))
}
