package abi

import (
	"fmt"

	"github.com/NilFoundation/nil/nil/cmd/nil/internal/common"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/spf13/cobra"
)

const pathFlag = "path"

func GetCommand() *cobra.Command {
	abiCmd := &cobra.Command{
		Use:          "abi",
		Short:        "Perform contract ABI encoding/decoding",
		SilenceUsage: true,
	}

	var path string

	encodeCmd := &cobra.Command{
		Use:          "encode [method] [args...]",
		Short:        "Enconde a contract call",
		Args:         cobra.MinimumNArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			abi, err := common.ReadAbiFromFile(path)
			if err != nil {
				return err
			}

			data, err := common.ArgsToCalldata(abi, args[0], args[1:])
			if err != nil {
				return err
			}

			fmt.Println(hexutil.Encode(data))
			return nil
		},
	}

	decodeCmd := &cobra.Command{
		Use:          "decode [method] [data]",
		Short:        "Decode the result of a contract call",
		Args:         cobra.MinimumNArgs(2),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := hexutil.DecodeHex(args[1])
			if err != nil {
				return err
			}

			abi, err := common.ReadAbiFromFile(path)
			if err != nil {
				return err
			}

			outputs, err := common.CalldataToArgs(abi, args[0], data)
			if err != nil {
				return err
			}

			for _, output := range outputs {
				fmt.Printf("%s: %v\n", output.Type, output.Value)
			}

			return nil
		},
	}

	abiCmd.PersistentFlags().StringVar(
		&path,
		pathFlag,
		"",
		"The path to the ABI file",
	)
	check.PanicIfErr(abiCmd.MarkPersistentFlagRequired(pathFlag))

	abiCmd.AddCommand(encodeCmd)
	abiCmd.AddCommand(decodeCmd)

	return abiCmd
}
