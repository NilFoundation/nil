package types

import (
	"github.com/NilFoundation/nil/nil/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
)

var (
	EmptyRootHash = common.Hash(ethtypes.EmptyRootHash)
	EmptyCodeHash = common.Hash(ethtypes.EmptyCodeHash)
)
