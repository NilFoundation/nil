package execution

import (
	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/core/types"
	lru "github.com/hashicorp/golang-lru/v2"
)

var FailureReceiptCache, _ = lru.New[common.Hash, *types.Receipt](1024)
