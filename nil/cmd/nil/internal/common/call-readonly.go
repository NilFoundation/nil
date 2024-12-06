package common

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/cliservice"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
)

type ResultHandler = func(res *jsonrpc.CallRes) ([]*ArgValue, []*NamedArgValues, error)

func formatArgValues(argValues []*ArgValue) error {
	for _, output := range argValues {
		outputStr, err := json.Marshal(output.Value)
		if err != nil {
			return err
		}
		fmt.Printf("%s: %s\n", output.Type, outputStr)
	}
	return nil
}

func CallReadonly(
	service *cliservice.Service,
	address types.Address,
	calldata []byte,
	handleResult ResultHandler,
	params *Params,
) error {
	var inOverrides *jsonrpc.StateOverrides
	if params.InOverridesPath != "" {
		inOverridesData, err := os.ReadFile(params.InOverridesPath)
		if err != nil {
			return err
		}

		if err := json.Unmarshal(inOverridesData, &inOverrides); err != nil {
			return err
		}
	}

	res, err := service.CallContract(address, params.FeeCredit, calldata, inOverrides)
	if err != nil {
		return err
	}

	outputs, logs, err := handleResult(res)
	if err != nil {
		return err
	}

	if params.OutOverridesPath != "" {
		outOverridesData, err := json.Marshal(res.StateOverrides)
		if err != nil {
			return err
		}

		if err := os.WriteFile(params.OutOverridesPath, outOverridesData, 0o600); err != nil {
			return err
		}
	}

	if len(outputs) == 0 {
		fmt.Println("Success, no result")
	} else {
		if !Quiet {
			fmt.Println("Success, result:")
		}
		if err := formatArgValues(outputs); err != nil {
			return err
		}
	}

	if params.WithDetails {
		if len(logs) > 0 {
			fmt.Println("Logs:")
			for _, logValues := range logs {
				fmt.Printf("Event: %s\n", logValues.Name)
				if err := formatArgValues(logValues.ArgValues); err != nil {
					return err
				}
			}
		}

		if len(res.DebugLogs) > 0 {
			fmt.Println("Debug logs:")
			for _, log := range res.DebugLogs {
				fmt.Print(log.Message)
				if len(log.Data) > 0 {
					fmt.Print(" ", log.Data)
				}
				fmt.Println()
			}
		}

		fmt.Printf("Coins used: %s\n", res.CoinsUsed)
		if len(res.OutMessages) > 0 {
			fmt.Println("Outbound messages:")
			messagesStr, err := json.MarshalIndent(res.OutMessages, "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(messagesStr))
		}
	}

	return nil
}
