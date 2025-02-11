package contracts

import (
	"bytes"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/concurrent"
	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/NilFoundation/nil/nil/contracts"
	"github.com/NilFoundation/nil/nil/internal/abi"
	"github.com/NilFoundation/nil/nil/internal/types"
)

const (
	NameSmartAccount  = "SmartAccount"
	NameFaucet        = "Faucet"
	NameFaucetToken   = "FaucetToken"
	NamePrecompile    = "__Precompile__"
	NameNilTokenBase  = "NilTokenBase"
	NameNilBounceable = "NilBounceable"
	NameNilConfigAbi  = "NilConfigAbi"
	NameL1BlockInfo   = "system/L1BlockInfo"
)

var (
	codeCache = concurrent.NewMap[string, types.Code]()
	abiCache  = concurrent.NewMap[string, *abi.ABI]()
)

func GetCode(name string) (types.Code, error) {
	// The result taken from the cache must be cloned.
	if res, ok := codeCache.Get(name); ok {
		return res.Clone(), nil
	}

	code, err := contracts.Fs.ReadFile("compiled/" + name + ".bin")
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

	data, err := contracts.Fs.ReadFile("compiled/" + name + ".abi")
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

func GetAbiData(name string) (string, error) {
	data, err := contracts.Fs.ReadFile("compiled/" + name + ".abi")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func CalculateAddress(name string, shardId types.ShardId, salt []byte, ctorArgs ...any) (types.Address, error) {
	code, err := GetCode(name)
	if err != nil {
		return types.Address{}, err
	}

	if len(ctorArgs) != 0 {
		argsPacked, err := NewCallData(name, "", ctorArgs...)
		if err != nil {
			return types.Address{}, err
		}
		code = append(code, argsPacked...)
	}
	payload := types.BuildDeployPayload(code, common.BytesToHash(salt))

	return types.CreateAddress(shardId, payload), nil
}

func NewCallData(fileName, methodName string, args ...any) ([]byte, error) {
	abiCallee, err := GetAbi(fileName)
	if err != nil {
		return nil, err
	}
	return abiCallee.Pack(methodName, args...)
}

func UnpackData(fileName, methodName string, data []byte) ([]interface{}, error) {
	abiCallee, err := GetAbi(fileName)
	if err != nil {
		return nil, err
	}
	return abiCallee.Unpack(methodName, data)
}
