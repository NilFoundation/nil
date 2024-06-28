package wallet

import (
	"errors"

	"github.com/NilFoundation/nil/cli/service"
	"github.com/NilFoundation/nil/client/rpc"
	"github.com/NilFoundation/nil/cmd/nil_cli/common"
	"github.com/NilFoundation/nil/common/logging"
	"github.com/NilFoundation/nil/core/types"
	"github.com/spf13/cobra"
)

var logger = logging.NewLogger("walletNewCommand")

var defaultNewWalletAmount = types.NewUint256(100_000_000)

func NewCommand(cfg *common.Config) *cobra.Command {
	serverCmd := &cobra.Command{
		Use:   "new",
		Short: "Create new wallet with initial value on the cluster",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runNew(cmd, args, cfg)
		},
		SilenceUsage: true,
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

	params.amount = *defaultNewWalletAmount
	cmd.Flags().Var(
		&params.amount,
		amountFlag,
		"Start balance (capped at 10'000'000). Deployment fee will be subtracted",
	)
}

func runNew(_ *cobra.Command, _ []string, cfg *common.Config) error {
	logger.Info().Msgf("RPC Endpoint: %s", cfg.RPCEndpoint)

	amount := &params.amount
	if amount.Cmp(&defaultNewWalletAmount.Int) > 0 {
		logger.Warn().
			Msgf("Specified balance (%s) is greater than a limit (%s). Decrease it.", &params.amount, defaultNewWalletAmount)
		amount = defaultNewWalletAmount
	}

	client := rpc.NewClient(cfg.RPCEndpoint)
	srv := service.NewService(client, cfg.PrivateKey)
	walletAddress, err := srv.CreateWallet(params.shardId, params.salt, amount, &cfg.PrivateKey.PublicKey)

	if errors.Is(err, service.ErrWalletExists) {
		logger.Error().Err(err).Msg("failed to create wallet")
		return nil
	}
	if err != nil {
		return err
	}

	if err := common.PatchConfig(map[string]interface{}{
		common.AddressField: walletAddress.Hex(),
	}, false); err != nil {
		logger.Error().Err(err).Msg("failed to update wallet address in config file")
	}

	logger.Info().Msgf("New wallet address: %s", walletAddress.Hex())
	return nil
}
