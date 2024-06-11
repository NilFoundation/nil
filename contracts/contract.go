package contracts

import (
	"bytes"

	"github.com/NilFoundation/nil/common/hexutil"
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
