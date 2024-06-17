package receipt

import (
	"errors"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/core/types"
)

var errNoSelected = errors.New("at least one flag (--hash) is required")

const (
	hashFlag    = "hash"
	shardIdFlag = "shard-id"
)

var params = &receiptParams{}

type receiptParams struct {
	hash    common.Hash
	shardId types.ShardId
}

// initRawParams validates all parameters to ensure they are correctly set
func (p *receiptParams) initRawParams() error {
	flagsSet := 0

	if p.hash != common.EmptyHash {
		flagsSet++
	}

	if flagsSet == 0 {
		return errNoSelected
	}

	return nil
}
