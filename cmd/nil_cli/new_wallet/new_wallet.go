package new_wallet

import (
	"crypto/ecdsa"

	"github.com/NilFoundation/nil/cli/service"
	"github.com/NilFoundation/nil/client/rpc"
	"github.com/NilFoundation/nil/cmd/nil_cli/config"
	"github.com/NilFoundation/nil/common/check"
	"github.com/NilFoundation/nil/common/hexutil"
	"github.com/NilFoundation/nil/common/logging"
	"github.com/NilFoundation/nil/contracts"
	"github.com/NilFoundation/nil/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/spf13/cobra"
)

var logger = logging.NewLogger("newWalletCommand")

func GetCommand(cfg *config.Config) *cobra.Command {
	serverCmd := &cobra.Command{
		Use:   "new-wallet",
		Short: "Create new wallet with initial value on the cluster",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return runPreRun(cmd, args, cfg)
		},
		Run: func(cmd *cobra.Command, args []string) {
			runCommand(cmd, args, cfg)
		},
	}

	setFlags(serverCmd)

	return serverCmd
}

type codeValue types.Code

func (c codeValue) String() string {
	if len(c) == 0 {
		return "<default wallet code>"
	}
	v := types.Code(c).Hex()
	return v
}

func (c *codeValue) Set(value string) error {
	*(*types.Code)(c) = hexutil.FromHex(value)
	return nil
}

func (*codeValue) Type() string {
	return "Code"
}

func newCodeValue(val types.Code, p *types.Code) *codeValue {
	*p = val
	return (*codeValue)(p)
}

func defaultWalletCode(privateKey *ecdsa.PrivateKey) types.Code {
	walletCode, err := contracts.GetCode("Wallet")
	check.PanicIfErr(err)

	walletAbi, err := contracts.GetAbi("Wallet")
	check.PanicIfErr(err)

	publicKey := crypto.CompressPubkey(&privateKey.PublicKey)
	args, err := walletAbi.Pack("", publicKey)
	check.PanicIfErr(err)

	return types.Code(append(walletCode, args...))
}

func setFlags(cmd *cobra.Command) {
	params.salt = *types.NewUint256(0)
	cmd.Flags().Var(
		&params.salt,
		saltFlag,
		"Salt for wallet address calculation")

	cmd.Flags().Var(
		newCodeValue(types.Code{}, &params.code),
		codeFlag,
		"Bytecode of wallet constructor")

	// TODO: Be able to create wallets in any shard.
	// cmd.Flags().Var(
	//	types.NewShardId(&params.shardId, types.BaseShardId),
	//	shardIdFlag,
	//	"Specify the shard id to interact with",
	//)
}

func runCommand(_ *cobra.Command, _ []string, cfg *config.Config) {
	logger.Info().Msgf("RPC Endpoint: %s", cfg.RPCEndpoint)

	client := rpc.NewClient(cfg.RPCEndpoint)
	service := service.NewService(client, cfg.PrivateKey)
	walletAddress, err := service.CreateWallet(params.shardId, params.code, params.salt, cfg.PrivateKey)
	check.PanicIfErr(err)

	logger.Info().Msgf("New wallet address: %s", walletAddress.Hex())
}

func runPreRun(_ *cobra.Command, _ []string, cfg *config.Config) error {
	return params.initRawParams(cfg)
}
