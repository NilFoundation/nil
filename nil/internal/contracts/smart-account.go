package contracts

import (
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/internal/types"
)

func PrepareDefaultSmartAccountForOwnerCode(publicKey []byte) types.Code {
	smartAccountCode, err := GetCode(NameSmartAccount)
	check.PanicIfErr(err)

	args, err := NewCallData(NameSmartAccount, "", publicKey)
	check.PanicIfErr(err)

	return append(smartAccountCode.Clone(), args...)
}
