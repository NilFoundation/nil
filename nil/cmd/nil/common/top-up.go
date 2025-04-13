package common

import (
	"context"
	"fmt"

	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/cliservice"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var titleCaser = cases.Title(language.English)

func RunTopUp(
	ctx context.Context, name string, cfg *Config, address types.Address, amount types.Value, tokId string, quiet bool,
) error {
	faucet, err := GetFaucetRpcClient()
	if err != nil {
		return err
	}
	service := cliservice.NewService(ctx, GetRpcClient(), cfg.PrivateKey, faucet)

	faucetAddress := types.FaucetAddress
	if len(tokId) == 0 {
		tokId = types.GetTokenName(types.TokenId(faucetAddress))
	} else {
		var ok bool
		tokens := types.GetTokens()
		faucetAddress, ok = tokens[tokId]
		if !ok {
			if err = faucetAddress.Set(tokId); err != nil {
				return fmt.Errorf("undefined token id: %s", tokId)
			}
		}
	}

	if _, err = service.GetBalance(address); err != nil {
		return err
	}

	if err = service.TopUpViaFaucet(faucetAddress, address, amount); err != nil {
		return err
	}

	var balance types.Value
	if faucetAddress == types.FaucetAddress {
		balance, err = service.GetBalance(address)
		if err != nil {
			return err
		}
	} else {
		tokens, err := service.GetTokens(address)
		if err != nil {
			return err
		}
		var ok bool
		balance, ok = tokens[types.TokenId(faucetAddress)]
		if !ok {
			return fmt.Errorf("token %s for %s %s is not found", faucetAddress, name, address)
		}
	}

	if !quiet {
		fmt.Printf("%s balance: ", titleCaser.String(name))
	}

	fmt.Print(balance)
	if !quiet && len(tokId) > 0 {
		fmt.Printf(" [%s]", tokId)
	}
	fmt.Println()

	return nil
}
