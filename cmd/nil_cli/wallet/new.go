package wallet

import (
	"fmt"

	"github.com/NilFoundation/nil/cli/service"
	"github.com/NilFoundation/nil/cmd/nil_cli/common"
	"github.com/NilFoundation/nil/cmd/nil_cli/config"
	"github.com/NilFoundation/nil/common/check"
	"github.com/NilFoundation/nil/common/logging"
	"github.com/NilFoundation/nil/core/types"
	"github.com/spf13/cobra"
)

var logger = logging.NewLogger("walletNewCommand")

var defaultNewWalletAmount = types.NewValueFromUint64(100_000_000)

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

	params.new_wallet_amount = defaultNewWalletAmount
	cmd.Flags().Var(
		&params.new_wallet_amount,
		amountFlag,
		"Start balance (capped at 10'000'000). Deployment fee will be subtracted",
	)
}

func runNew(_ *cobra.Command, _ []string, cfg *common.Config) error {
	amount := params.new_wallet_amount
	if amount.Cmp(defaultNewWalletAmount) > 0 {
		logger.Warn().
			Msgf("Specified balance (%s) is greater than a limit (%s). Decrease it.", &params.new_wallet_amount, defaultNewWalletAmount)
		amount = defaultNewWalletAmount
	}

	srv := service.NewService(common.GetRpcClient(), cfg.PrivateKey)
	check.PanicIfNotf(cfg.PrivateKey != nil, "Private key doesn't set in config")
	walletAddress, err := srv.CreateWallet(params.shardId, &params.salt, amount, &cfg.PrivateKey.PublicKey)
	if err != nil {
		return err
	}

	if err := common.PatchConfig(map[string]interface{}{
		common.AddressField: walletAddress.Hex(),
	}, false); err != nil {
		logger.Error().Err(err).Msg("failed to update wallet address in config file")
	}

	if !config.Quiet {
		fmt.Print("New wallet address: ")
	}
	fmt.Println(walletAddress.Hex())
	return nil
}
