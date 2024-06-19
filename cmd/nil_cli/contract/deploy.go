package contract

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"os"

	"github.com/NilFoundation/nil/cli/service"
	"github.com/NilFoundation/nil/client/rpc"
	"github.com/NilFoundation/nil/cmd/nil_cli/config"
	"github.com/NilFoundation/nil/core/types"
	"github.com/spf13/cobra"
)

func GetDeployCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deploy [path to file]",
		Short: "Deploy smart contract",
		Long:  "Deploy smart contract with specified hex-bytecode from stdin or from file",
		Args:  cobra.MaximumNArgs(1),
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
}

func runDeploy(_ *cobra.Command, args []string, cfg *config.Config) error {
	var bytecode []byte
	var err error

	if len(args) == 1 {
		bytecode, err = os.ReadFile(args[0])
		if err != nil {
			return fmt.Errorf("failed to read file: %w", err)
		}
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

	client := rpc.NewClient(cfg.RPCEndpoint)
	service := service.NewService(client, cfg.PrivateKey)
	_, _, err = service.DeployContract(params.shardId, cfg.Address, bytecode)
	if err != nil {
		return err
	}
	return nil
}
