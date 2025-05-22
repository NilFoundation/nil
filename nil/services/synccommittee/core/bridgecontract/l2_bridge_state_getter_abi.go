package bridgecontract

import (
	"bytes"
	_ "embed"

	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/internal/abi"
)

//go:embed IL2BridgeStateGetter.abi
var l2BridgeStateGetterContractABIData []byte

var l2BridgeStateGetterContractABI *abi.ABI

func init() {
	abi, err := abi.JSON(bytes.NewReader(l2BridgeStateGetterContractABIData))
	check.PanicIfErr(err)
	if err != nil {
		panic(err)
	}
	l2BridgeStateGetterContractABI = &abi
}

func GetL2BridgeStateGetterABI() *abi.ABI {
	return l2BridgeStateGetterContractABI
}
