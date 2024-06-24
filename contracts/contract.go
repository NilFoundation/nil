package contracts

import (
	"bytes"

	"github.com/NilFoundation/nil/common/concurrent"
	"github.com/NilFoundation/nil/common/hexutil"
	"github.com/NilFoundation/nil/core/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
)

var (
	codeCache = concurrent.NewMap[string, types.Code]()
	abiCache  = concurrent.NewMap[string, *abi.ABI]()
)

func GetCode(name string) ([]byte, error) {
	// The result taken from the cache must be cloned.
	if res, ok := codeCache.Get(name); ok {
		return res.Clone(), nil
	}

	code, err := Fs.ReadFile("compiled/" + name + ".bin")
	if err != nil {
		return nil, err
	}

	res := types.Code(hexutil.FromHex(string(code)))
	codeCache.Put(name, res)
	return res.Clone(), nil
}

func GetAbi(name string) (*abi.ABI, error) {
	if res, ok := abiCache.Get(name); ok {
		return res, nil
	}

	data, err := Fs.ReadFile("compiled/" + name + ".abi")
	if err != nil {
		return nil, err
	}

	res, err := abi.JSON(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	abiCache.Put(name, &res)
	return &res, nil
}

func CalculateAddress(name string, shardId types.ShardId, ctorArgs []any, salt []byte) (types.Address, error) {
	code, err := GetCode(name)
	if err != nil {
		return types.Address{}, err
	}
	if len(ctorArgs) != 0 {
		abi, err := GetAbi(name)
		if err != nil {
			return types.Address{}, err
		}
		argsPacked, err := abi.Pack("", ctorArgs...)
		if err != nil {
			return types.Address{}, err
		}
		code = append(code, argsPacked...)
	}
	code = append(code, salt...)

	return types.CreateAddress(shardId, code), nil
}
