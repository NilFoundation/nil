package contract

import (
	"errors"

	"github.com/NilFoundation/nil/core/types"
)

var (
	errNoSelected       = errors.New("at least one flag (--deploy, --code) is required")
	errMultipleSelected = errors.New("only one flag (--deploy or --code) can be set")
)

const (
	deployFlag   = "deploy"
	codeFlag     = "code"
	addressFlag  = "address"
	bytecodeFlag = "bytecode"
	shardIdFlag  = "shard-id"
)

var params = &contractParams{}

type contractParams struct {
	deploy   string
	code     string
	address  string
	bytecode string
	shardId  types.ShardId
}

// initRawParams validates all parameters to ensure they are correctly set
func (p *contractParams) initRawParams() error {
	flagsSet := 0

	if p.deploy != "" {
		flagsSet++
	}

	if p.code != "" {
		flagsSet++
	}

	if p.address != "" && p.bytecode != "" {
		flagsSet++
	}

	if (p.address != "" && p.bytecode == "") || (p.address == "" && p.bytecode != "") {
		return errors.New("both --address and --bytecode must be set together")
	}

	if p.shardId == 0 {
		return errors.New("--shard-id must be set and non-zero")
	}

	if flagsSet == 0 {
		return errNoSelected
	}
	if flagsSet > 1 {
		return errMultipleSelected
	}

	return nil
}
