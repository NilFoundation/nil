package common

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/cliservice"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
)

type ResultHandler = func(res *jsonrpc.CallRes) ([]*ArgValue, error)

func CallReadonly(
	service *cliservice.Service,
	address types.Address,
	calldata []byte,
	feeCredit types.Value,

	handleResult ResultHandler,
	inOverridesPath string,
	outOverridesPath string,
	withDetails bool,
	quiet bool,
) error {
	var inOverrides *jsonrpc.StateOverrides
	if inOverridesPath != "" {
		inOverridesData, err := os.ReadFile(inOverridesPath)
		if err != nil {
			return err
		}

		if err := json.Unmarshal(inOverridesData, &inOverrides); err != nil {
			return err
		}
	}

	res, err := service.CallContract(address, feeCredit, calldata, inOverrides)
	if err != nil {
		return err
	}

	outputs, err := handleResult(res)
	if err != nil {
		return err
	}

	if outOverridesPath != "" {
		outOverridesData, err := json.Marshal(res.StateOverrides)
		if err != nil {
			return err
		}

		if err := os.WriteFile(outOverridesPath, outOverridesData, 0o600); err != nil {
			return err
		}
	}

	if len(outputs) == 0 {
		fmt.Println("Success, no result")
	} else {
		if !quiet {
			fmt.Println("Success, result:")
		}
		for _, output := range outputs {
			fmt.Printf("%s: %v\n", output.Type, output.Value)
		}
	}

	if withDetails {
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
