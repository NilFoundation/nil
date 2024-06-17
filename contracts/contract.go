package contracts

import (
	"bytes"

	"github.com/NilFoundation/nil/common/hexutil"
	"github.com/NilFoundation/nil/core/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
)

func GetCode(name string) ([]byte, error) {
	code, err := Fs.ReadFile("compiled/" + name + ".bin")
	if err != nil {
		return nil, err
	}
	return hexutil.FromHex(string(code)), nil
}

func GetAbi(name string) (*abi.ABI, error) {
	data, err := Fs.ReadFile("compiled/" + name + ".abi")
	if err != nil {
		return nil, err
	}
	abi, err := abi.JSON(bytes.NewReader(data))
	return &abi, err
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
