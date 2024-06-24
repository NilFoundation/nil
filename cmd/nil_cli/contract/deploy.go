package contract

import (
	"bufio"
	"encoding/hex"
	"errors"
	"fmt"
	"os"

	"github.com/NilFoundation/nil/cli/service"
	"github.com/NilFoundation/nil/client/rpc"
	"github.com/NilFoundation/nil/cmd/nil_cli/config"
	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/hexutil"
	"github.com/NilFoundation/nil/common/logging"
	"github.com/NilFoundation/nil/core/types"
	"github.com/spf13/cobra"
)

var logger = logging.NewLogger("contractDeployCommand")

func GetDeployCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deploy [path to file] [args...]",
		Short: "Deploy smart contract",
		Long:  "Deploy smart contract with specified hex-bytecode from stdin or from file",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDeploy(cmd, args, cfg)
		},
	}

	setDeployFlags(cmd)

	return cmd
}

func setDeployFlags(cmd *cobra.Command) {
	cmd.Flags().Var(
		types.NewShardId(&params.shardId, types.BaseShardId),
		shardIdFlag,
		"Specify the shard id to interact with",
	)

	params.salt = *types.NewUint256(0)
	cmd.Flags().Var(
		&params.salt,
		saltFlag,
		"Salt for deploy message",
	)

	cmd.Flags().StringVar(
		&params.abiPath,
		abiFlag,
		"",
		"Path to ABI file",
	)
}

func runDeploy(_ *cobra.Command, args []string, cfg *config.Config) error {
	if cfg.Address.Equal(types.EmptyAddress) {
		logger.Error().Msg("config.address is empty")
		return errors.New("deploy failed")
	}
	logger.Info().Msgf("Deploy via wallet: %s", cfg.Address)
	var bytecode []byte
	var err error

	client := rpc.NewClient(cfg.RPCEndpoint)
	service := service.NewService(client, cfg.PrivateKey)

	if len(args) > 0 {
		codeHex, err := os.ReadFile(args[0])
		if err != nil {
			return fmt.Errorf("failed to read file: %w", err)
		}

		bytecode = hexutil.FromHex(string(codeHex))
		if params.abiPath != "" {
			calldata, err := service.ArgsToCalldata(params.abiPath, "", args[1:])
			if err != nil {
				return fmt.Errorf("failed to handle constructor arguments: %w", err)
			}
			bytecode = append(bytecode, calldata...)
		}
		bytecode = types.BuildDeployPayload(bytecode, common.Hash(params.salt.Bytes32()))
	} else {
		scanner := bufio.NewScanner(os.Stdin)
		input := ""
		for scanner.Scan() {
			input += scanner.Text()
		}
		if err := scanner.Err(); err != nil {
			return fmt.Errorf("failed to read from stdin: %w", err)
		}
		bytecode, err = hex.DecodeString(input)
		if err != nil {
			return fmt.Errorf("failed to decode hex: %w", err)
		}
	}

	_, _, err = service.DeployContract(params.shardId, cfg.Address, bytecode, nil)
	if err != nil {
		return err
	}
	return nil
}
