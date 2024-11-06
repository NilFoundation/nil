package common

import (
	"github.com/NilFoundation/nil/nil/client/rpc"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/common/version"
	"github.com/NilFoundation/nil/nil/services/cometa"
	"github.com/NilFoundation/nil/nil/services/faucet"
	"github.com/rs/zerolog"
)

var (
	client       *rpc.Client
	cometaClient *cometa.Client
	faucetClient *faucet.Client
)

func InitRpcClient(cfg *Config, logger zerolog.Logger) {
	client = rpc.NewClientWithDefaultHeaders(
		cfg.RPCEndpoint,
		logger,
		map[string]string{
			"User-Agent": "nil-cli/" + version.GetGitRevision(),
		},
	)

	if cfg.CometaEndpoint != "" {
		cometaClient = cometa.NewClient(cfg.CometaEndpoint)
	}

	if cfg.FaucetEndpoint != "" {
		faucetClient = faucet.NewClient(cfg.FaucetEndpoint)
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

func GetFaucetRpcClient() *faucet.Client {
	check.PanicIfNotf(faucetClient != nil && faucetClient.IsValid(), "faucet client is not valid")
	return faucetClient
}
