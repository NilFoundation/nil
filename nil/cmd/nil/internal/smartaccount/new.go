package smartaccount

import (
	"fmt"

	"github.com/NilFoundation/nil/nil/cmd/nil/common"
	"github.com/NilFoundation/nil/nil/cmd/nil/internal/config"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/cliservice"
	"github.com/spf13/cobra"
)

var defaultNewSmartAccountAmount = types.GasToValue(1_000_000_000)

func NewCommand(cfg *common.Config) *cobra.Command {
	serverCmd := &cobra.Command{
		Use:   "new",
		Short: "Create a new smart account with some initial balance on the cluster",
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
		"The salt for the smart account address calculation")

	cmd.Flags().Var(
		types.NewShardId(&params.shardId, types.BaseShardId),
		shardIdFlag,
		"Specify the shard ID to interact with",
	)

	cmd.Flags().Var(
		&params.Fee.FeeCredit,
		feeCreditFlag,
		"The fee credit for smart account creation. If set to 0, it will be estimated automatically",
	)

	params.newSmartAccountAmount = defaultNewSmartAccountAmount
	cmd.Flags().Var(
		&params.newSmartAccountAmount,
		amountFlag,
		"The initial balance (capped at 10'000'000). The deployment fee will be subtracted from this balance",
	)
}

func runNew(cmd *cobra.Command, _ []string, cfg *common.Config) error {
	amount := params.newSmartAccountAmount
	if amount.Cmp(defaultNewSmartAccountAmount) > 0 {
		logger.Warn().Msgf(
			"The specified balance (%s) is greater than the limit (%s). Decrease it.",
			&params.newSmartAccountAmount, defaultNewSmartAccountAmount)
		amount = defaultNewSmartAccountAmount
	}

	faucet, err := common.GetFaucetRpcClient()
	if err != nil {
		return err
	}
	srv := cliservice.NewService(cmd.Context(), common.GetRpcClient(), cfg.PrivateKey, faucet)
	check.PanicIfNotf(cfg.PrivateKey != nil, "A private key is not set in the config file")
	smartAccountAddress, err := srv.CreateSmartAccount(params.shardId, &params.salt, amount,
		types.NewFeePackFromFeeCredit(params.Fee.FeeCredit), &cfg.PrivateKey.PublicKey)
	if err != nil {
		return err
	}

	if err := config.PatchConfig(map[string]interface{}{
		config.AddressField: smartAccountAddress.Hex(),
	}, false); err != nil {
		logger.Error().Err(err).Msg("failed to update the smart account address in the config file")
	}

	if !common.Quiet {
		fmt.Print("New smart account address: ")
	}
	fmt.Println(smartAccountAddress.Hex())
	return nil
}
