package keygen

import (
	"fmt"

	"github.com/NilFoundation/nil/nil/cmd/nil/internal/common"
	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/NilFoundation/nil/nil/services/cliservice"
	"github.com/spf13/cobra"
)

var asYaml bool

const asYamlFlag = "yaml"

func NewP2pCommand(keygen *cliservice.Service) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "new-p2p",
		Short: "Generate a new p2p key",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runNewP2p(cmd, args, keygen)
		},
		SilenceUsage: true,
	}
	cmd.Flags().BoolVar(
		&asYaml,
		asYamlFlag,
		false,
		"Output as YAML (network_keys.yaml format)",
	)
	return cmd
}

func runNewP2p(_ *cobra.Command, _ []string, keygen *cliservice.Service) error {
	privateKey, pubKey, identity, err := keygen.GenerateNewP2pKey()
	if err != nil {
		return err
	}

	if asYaml {
		fmt.Printf("privateKey: \"0x%x\"\npublicKey: \"0x%x\"\nidentity: \"%s\"\n", privateKey, pubKey, identity)
		return nil
	}

	if !common.Quiet {
		fmt.Printf("Private key: ")
	}
	fmt.Println(hexutil.Encode(privateKey))

	if !common.Quiet {
		fmt.Printf("Public key: ")
	}
	fmt.Println(hexutil.Encode(pubKey))

	if !common.Quiet {
		fmt.Printf("Identity: ")
	}
	fmt.Println(identity)
	return nil
}
