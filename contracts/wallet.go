package contracts

import (
	"github.com/NilFoundation/nil/common/check"
	"github.com/NilFoundation/nil/core/types"
)

func PrepareDefaultWalletForOwnerCode(publicKey []byte) types.Code {
	walletCode, err := GetCode("Wallet")
	check.PanicIfErr(err)

	walletAbi, err := GetAbi("Wallet")
	check.PanicIfErr(err)

	args, err := walletAbi.Pack("", publicKey)
	check.PanicIfErr(err)

	return types.Code(append(walletCode, args...))
}
