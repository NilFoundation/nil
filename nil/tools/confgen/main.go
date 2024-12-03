package main

import (
	"flag"
	"fmt"
	"os"
	"path"

	"github.com/NilFoundation/nil/nil/cmd/nild/nildconfig"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/network"
	"github.com/NilFoundation/nil/nil/services/nilservice"
	"github.com/libp2p/go-libp2p/core/peer"
	"gopkg.in/yaml.v3"
)

func main() {
	nShards := flag.Uint("n", 3, "number of shards")
	dir := flag.String("dir", ".", "output directory")

	flag.Parse()

	check.PanicIfErr(do(*nShards, *dir))
}

func shardSuffix(nShards, id uint) string {
	return fmt.Sprintf("_%d_%d", nShards, id)
}

func do(nShards uint, dir string) error {
	logger := logging.NewLogger("confgen")

	logger.Info().Msgf("Generating %d configs in %s", nShards, dir)

	configs := make([]*nildconfig.Config, nShards)
	peerAddresses := make(network.AddrInfoSlice, nShards)

	for i := range nShards {
		suffix := shardSuffix(nShards, i)
		networkKeysFileName := "network-keys" + suffix + ".yaml"
		mainKeysFileName := "main-keys" + suffix + ".yaml"
		dbPath := "test.db" + suffix

		cfg := &nildconfig.Config{
			Config: &nilservice.Config{
				NShards:         uint32(nShards),
				MyShards:        []uint{i},
				SplitShards:     true,
				MainKeysOutPath: mainKeysFileName,
				NetworkKeysPath: networkKeysFileName,
				RPCPort:         9000 + int(i),
				Network: &network.Config{
					TcpPort:           19000 + int(i),
					DHTEnabled:        true,
					DHTBootstrapPeers: peerAddresses,
				},
			},
			DB: &db.BadgerDBOptions{
				Path: dbPath,
			},
		}

		key, err := network.GenerateAndDumpKeys(path.Join(dir, networkKeysFileName))
		if err != nil {
			return err
		}
		peerId, err := peer.IDFromPrivateKey(key)
		if err != nil {
			return err
		}

		peerAddress, err := peer.AddrInfoFromString(fmt.Sprintf("/ip4/127.0.0.1/tcp/%d/p2p/%s", cfg.Network.TcpPort, peerId))
		if err != nil {
			return err
		}
		peerAddresses[i] = network.AddrInfo(*peerAddress)

		configs[i] = cfg
	}

	for i, c := range configs {
		data, err := yaml.Marshal(c)
		if err != nil {
			return err
		}

		name := path.Join(dir, "config"+shardSuffix(nShards, uint(i))+".yaml")
		if err := os.WriteFile(name, data, 0o600); err != nil {
			return err
		}

		logger.Info().
			Uint(logging.FieldShardId, c.MyShards[0]).
			Msgf("Config for written to %s", name)
	}

	return nil
}
