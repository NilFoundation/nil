package wallet

import (
	"fmt"

	"github.com/NilFoundation/nil/nil/cmd/nil/internal/common"
	"github.com/NilFoundation/nil/nil/cmd/nil/internal/config"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/cliservice"
	"github.com/spf13/cobra"
)

var defaultNewWalletAmount = types.NewValueFromUint64(100_000_000)

func NewCommand(cfg *common.Config) *cobra.Command {
	serverCmd := &cobra.Command{
		Use:   "new",
		Short: "Create a new wallet with some initial balance on the cluster",
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
		"The salt for the wallet address calculation")

	cmd.Flags().Var(
		types.NewShardId(&params.shardId, types.BaseShardId),
		shardIdFlag,
		"Specify the shard ID to interact with",
	)

	cmd.Flags().Var(
		&params.feeCredit,
		feeCreditFlag,
		"The fee credit for wallet creation. If set to 0, it will be estimated automatically",
	)

	params.newWalletAmount = defaultNewWalletAmount
	cmd.Flags().Var(
		&params.newWalletAmount,
		amountFlag,
		"The initial balance (capped at 10'000'000). The deployment fee will be subtracted from this balance",
	)
}

func runNew(_ *cobra.Command, _ []string, cfg *common.Config) error {
	amount := params.newWalletAmount
	if amount.Cmp(defaultNewWalletAmount) > 0 {
		logger.Warn().
			Msgf("The specified balance (%s) is greater than the limit (%s). Decrease it.", &params.newWalletAmount, defaultNewWalletAmount)
		amount = defaultNewWalletAmount
	}

	faucet, err := common.GetFaucetRpcClient()
	if err != nil {
		return err
	}
	srv := cliservice.NewService(common.GetRpcClient(), cfg.PrivateKey, faucet)
	check.PanicIfNotf(cfg.PrivateKey != nil, "A private key is not set in the config file")
	walletAddress, err := srv.CreateWallet(params.shardId, &params.salt, amount, params.feeCredit, &cfg.PrivateKey.PublicKey)
	if err != nil {
		return err
	}

	if err := common.PatchConfig(map[string]interface{}{
		common.AddressField: walletAddress.Hex(),
	}, false); err != nil {
		logger.Error().Err(err).Msg("failed to update the wallet address in the config file")
	}

	if !config.Quiet {
		fmt.Print("New wallet address: ")
	}
	fmt.Println(walletAddress.Hex())
	return nil
}
