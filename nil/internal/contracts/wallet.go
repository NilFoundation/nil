package contracts

import (
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/internal/types"
)

func PrepareDefaultWalletForOwnerCode(publicKey []byte) types.Code {
	walletCode, err := GetCode(NameWallet)
	check.PanicIfErr(err)

	args, err := NewCallData(NameWallet, "", publicKey)
	check.PanicIfErr(err)

	return append(walletCode, args...)
}
