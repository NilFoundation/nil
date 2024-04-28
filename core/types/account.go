package types

import (
	common "github.com/NilFoundation/nil/common"
	"github.com/holiman/uint256"
)

type SmartContract struct {
	Address     common.Address
	Initialised bool
	Balance     uint256.Int
	StorageRoot common.Hash
	CodeHash    common.Hash
}
