package main

import (
	"context"
	"fmt"
	"os"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/network"
	"github.com/NilFoundation/nil/nil/services/rpc"
	"github.com/NilFoundation/nil/nil/services/rpc/httpcfg"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
	"github.com/NilFoundation/nil/nil/services/rpc/rawapi"
	"github.com/NilFoundation/nil/nil/services/rpc/transport"
)

// TODO: unify with nilservice
func startRpcServer(ctx context.Context, rawApi rawapi.Api, db db.ReadOnlyDB) error {
	logger := logging.NewLogger("RPC")

	port := 8530
	addr := fmt.Sprintf("tcp://127.0.0.1:%d", port)

	httpConfig := &httpcfg.HttpCfg{
		Enabled:         true,
		HttpURL:         addr,
		HttpCompression: true,
		TraceRequests:   true,
		HTTPTimeouts:    httpcfg.DefaultHTTPTimeouts,
	}

	debugImpl := jsonrpc.NewDebugAPI(rawApi, db, logger)

	apiList := []transport.API{
		{
			Namespace: "debug",
			Public:    true,
			Service:   jsonrpc.DebugAPI(debugImpl),
			Version:   "1.0",
		},
	}

	return rpc.StartRpcServer(ctx, httpConfig, apiList, logger)
}

func createNetworkManager(ctx context.Context) (*network.Manager, error) {
	conf := &network.Config{}
	conf.IPV4Address = "127.0.0.1"
	conf.TcpPort = 1051
	var err error
	if conf.PrivateKey, err = network.LoadOrGenerateKeys("rpc-node-network-keys.yaml"); err != nil {
		return nil, err
	}
	return network.NewManager(ctx, conf)
}

func main() {
	logger := logging.NewLogger("nilrpcd")
	logging.SetupGlobalLogger("debug")

	if len(os.Args) != 2 {
		logger.Error().Msgf("Usage: %s <libp2p server address>", os.Args[0])
		os.Exit(1)
	}
	serverAddress := os.Args[1]

	ctx := context.Background()
	clientNetworkManager, err := createNetworkManager(ctx)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to create network manager")
		os.Exit(1)
	}
	// TODO: shard router proxy
	rawApi, err := rawapi.NewNetworkRawApiAccessor(ctx, clientNetworkManager, serverAddress)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to create raw API accessor")
		os.Exit(1)
	}

	err = startRpcServer(ctx, rawApi, nil)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to start RPC server")
		os.Exit(1)
	}
}
