package l2

import (
	"bytes"
	_ "embed"

	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/internal/abi"
)

//go:embed L2BridgeMessenger.json.abi
var l2BridgeMessengerContractABIData []byte

var l2BridgeMessengerContractABI *abi.ABI

func init() {
	abi, err := abi.JSON(bytes.NewReader(l2BridgeMessengerContractABIData))
	check.PanicIfErr(err)
	if err != nil {
		panic(err)
	}
	l2BridgeMessengerContractABI = &abi
}

func GetL2BridgeMessengerABI() *abi.ABI {
	return l2BridgeMessengerContractABI
}
