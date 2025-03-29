package common

import (
	"errors"

	"github.com/NilFoundation/nil/nil/client/rpc"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/common/version"
	"github.com/NilFoundation/nil/nil/services/cometa"
	"github.com/NilFoundation/nil/nil/services/faucet"
)

var (
	client       *rpc.Client
	cometaClient *cometa.Client
	faucetClient *faucet.Client
)

func InitRpcClient(cfg *Config, logger logging.Logger) {
	client = rpc.NewClientWithDefaultHeaders(
		cfg.RPCEndpoint,
		logger,
		map[string]string{
			"User-Agent": "nil-cli/" + version.GetGitRevCount(),
		},
	)

	if cfg.CometaEndpoint != "" {
		cometaClient = cometa.NewClient(cfg.CometaEndpoint)
	} else {
		// Assuming that Cometa is running on the same endpoint as RPC
		cometaClient = cometa.NewClient(cfg.RPCEndpoint)
	}

	if cfg.FaucetEndpoint != "" {
		faucetClient = faucet.NewClient(cfg.FaucetEndpoint)
	} else {
		// Assuming that Faucet is running on the same endpoint as RPC
		faucetClient = faucet.NewClient(cfg.RPCEndpoint)
	}
}

func GetRpcClient() *rpc.Client {
	check.PanicIfNot(client != nil)
	return client
}

func GetCometaRpcClient() *cometa.Client {
	check.PanicIfNotf(cometaClient != nil && cometaClient.IsValid(), "cometa client is not valid")
	return cometaClient
}

func GetFaucetRpcClient() (*faucet.Client, error) {
	if faucetClient == nil || !faucetClient.IsValid() {
		return nil, errors.New("valid faucet client is not set (check config)")
	}
	return faucetClient, nil
}
