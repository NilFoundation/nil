package contract

import (
	"errors"

	"github.com/NilFoundation/nil/common/hexutil"
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
	abiFlag      = "abi"
)

var params = &contractParams{}

type contractParams struct {
	code     string
	address  types.Address
	bytecode hexutil.Bytes
	shardId  types.ShardId
	abiPath  string
}

// initRawParams validates all parameters to ensure they are correctly set
func (p *contractParams) initRawParams() error {
	flagsSet := 0

	if p.code != "" {
		flagsSet++
	}

	if p.address != types.EmptyAddress && p.bytecode != nil {
		flagsSet++
	}

	if (p.address != types.EmptyAddress && p.bytecode == nil) || (p.address == types.EmptyAddress && p.bytecode != nil) {
		return errors.New("both --address and --bytecode must be set together")
	}

	if flagsSet == 0 {
		return errNoSelected
	}
	if flagsSet > 1 {
		return errMultipleSelected
	}

	return nil
}
