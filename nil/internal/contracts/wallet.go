package contracts

import (
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/internal/types"
	"math/big"
)

func PrepareDefaultWalletForOwnerCode(publicKey []byte) types.Code {
	walletCode, err := GetCode(NameWallet)
	check.PanicIfErr(err)

	args, err := NewCallData(NameWallet, "", publicKey, big.NewInt(0), "", types.EmptyAddress)
	check.PanicIfErr(err)

	return append(walletCode, args...)
}
