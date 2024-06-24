package wallet

import (
	"errors"

	"github.com/NilFoundation/nil/cli/service"
	"github.com/NilFoundation/nil/client/rpc"
	"github.com/NilFoundation/nil/cmd/nil_cli/config"
	"github.com/NilFoundation/nil/common/logging"
	"github.com/NilFoundation/nil/core/types"
	"github.com/spf13/cobra"
)

var logger = logging.NewLogger("walletNewCommand")

func NewCommand(cfg *config.Config) *cobra.Command {
	serverCmd := &cobra.Command{
		Use:   "new",
		Short: "Create new wallet with initial value on the cluster",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runNew(cmd, args, cfg)
		},
	}

	setFlags(serverCmd)

	return serverCmd
}

func setFlags(cmd *cobra.Command) {
	params.salt = *types.NewUint256(0)
	cmd.Flags().Var(
		&params.salt,
		saltFlag,
		"Salt for wallet address calculation")

	cmd.Flags().Var(
		types.NewShardId(&params.shardId, types.BaseShardId),
		shardIdFlag,
		"Specify the shard id to interact with",
	)
}

func runNew(_ *cobra.Command, _ []string, cfg *config.Config) error {
	logger.Info().Msgf("RPC Endpoint: %s", cfg.RPCEndpoint)

	client := rpc.NewClient(cfg.RPCEndpoint)
	srv := service.NewService(client, cfg.PrivateKey)
	walletAddress, err := srv.CreateWallet(params.shardId, params.salt, &cfg.PrivateKey.PublicKey)

	if errors.Is(err, service.ErrWalletExists) {
		logger.Error().Err(err).Msg("failed to create wallet")
		return nil
	}
	if err != nil {
		return err
	}

	logger.Info().Msgf("New wallet address: %s", walletAddress.Hex())
	return nil
}
