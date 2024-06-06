package message

import (
	"errors"

	"github.com/NilFoundation/nil/core/types"
)

var errNoSelected = errors.New("at least one flag (--hash) is required")

const (
	hashFlag    = "hash"
	shardIdFlag = "shard-id"
)

var params = &messageParams{}

type messageParams struct {
	hash    string
	shardId types.ShardId
}

// initRawParams validates all parameters to ensure they are correctly set
func (p *messageParams) initRawParams() error {
	flagsSet := 0

	if p.hash != "" {
		flagsSet++
	}

	if p.shardId == 0 {
		return errors.New("--shard-id must be set and non-zero")
	}

	if flagsSet == 0 {
		return errNoSelected
	}

	return nil
}
