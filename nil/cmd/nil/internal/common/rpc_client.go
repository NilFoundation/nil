package common

import (
	"github.com/NilFoundation/nil/nil/client/rpc"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/common/version"
	"github.com/NilFoundation/nil/nil/services/cometa"
	"github.com/rs/zerolog"
)

var (
	client       *rpc.Client
	cometaClient *cometa.Client
)

func InitRpcClient(cfg *Config, logger zerolog.Logger) {
	client = rpc.NewClientWithDefaultHeaders(
		cfg.RPCEndpoint,
		logger,
		map[string]string{
			"User-Agent": "nil-cli/" + version.GetGitRevision(),
		},
	)

	cometaClient = cometa.NewClient(cfg.CometaEndpoint)
}

func GetRpcClient() *rpc.Client {
	check.PanicIfNot(client != nil)
	return client
}

func GetCometaRpcClient() *cometa.Client {
	check.PanicIfNotf(cometaClient != nil && cometaClient.IsValid(), "cometa client is not valid")
	return cometaClient
}
