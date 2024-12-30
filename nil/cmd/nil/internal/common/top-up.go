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
	ctx context.Context, name string, cfg *Config, address types.Address, amount types.Value, currId string, quiet bool,
) error {
	faucet, err := GetFaucetRpcClient()
	if err != nil {
		return err
	}
	service := cliservice.NewService(ctx, GetRpcClient(), cfg.PrivateKey, faucet)

	faucetAddress := types.FaucetAddress
	if len(currId) == 0 {
		currId = types.GetCurrencyName(types.CurrencyId(faucetAddress))
	} else {
		var ok bool
		currencies := types.GetCurrencies()
		faucetAddress, ok = currencies[currId]
		if !ok {
			if err = faucetAddress.Set(currId); err != nil {
				return fmt.Errorf("undefined currency id: %s", currId)
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
		currencies, err := service.GetCurrencies(address)
		if err != nil {
			return err
		}
		var ok bool
		balance, ok = currencies[types.CurrencyId(faucetAddress)]
		if !ok {
			return fmt.Errorf("Currency %s for %s %s is not found", faucetAddress, name, address)
		}
	}

	if !quiet {
		fmt.Printf("%s balance: ", titleCaser.String(name))
	}

	fmt.Print(balance)
	if !quiet && len(currId) > 0 {
		fmt.Printf(" [%s]", currId)
	}
	fmt.Println()

	return nil
}
