package common

import (
	"bufio"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"reflect"
	"strings"

	"github.com/NilFoundation/nil/common/hexutil"
	"github.com/ethereum/go-ethereum/accounts/abi"
	eth_common "github.com/ethereum/go-ethereum/common"
)

func PrepareArgs(abiPath string, args []string) ([]byte, error) {
	var calldata []byte
	if strings.HasPrefix(args[0], "0x") && abiPath == "" {
		calldata = hexutil.FromHex(args[0])
	} else {
		var err error
		calldata, err = ArgsToCalldata(abiPath, args[0], args[1:])
		if err != nil {
			return nil, err
		}
	}
	return calldata, nil
}

func parseCallArguments(args []string, inputs abi.Arguments) ([]any, error) {
	parsedArgs := make([]any, 0, len(args))
	if len(args) != len(inputs) {
		return nil, fmt.Errorf("invalid amout of arguments is provided: expected %d but got %d", len(inputs), len(args))
	}

	for ind, arg := range args {
		tp := inputs[ind].Type
		refTp := tp.GetType()
		val := reflect.New(refTp).Elem()
		switch tp.T {
		case abi.IntTy:
			fallthrough
		case abi.UintTy:
			i, ok := new(big.Int).SetString(arg, 0)
			if !ok {
				return nil, fmt.Errorf("failed to parse int argument: %s", arg)
			}
			if tp.Size > 64 {
				val.Set(reflect.ValueOf(i))
			} else {
				if tp.T == abi.UintTy {
					val.SetUint(i.Uint64())
				} else {
					val.SetInt(i.Int64())
				}
			}
		case abi.StringTy:
			val.SetString(arg)
		case abi.AddressTy:
			var address eth_common.Address
			if err := address.UnmarshalText([]byte(arg)); err != nil {
				return nil, fmt.Errorf("failed to parse address argument: %w", err)
			}
			val.Set(reflect.ValueOf(address))
		default:
			return nil, fmt.Errorf("unsupported argument type: %s", tp.String())
		}
		parsedArgs = append(parsedArgs, val.Interface())
	}
	return parsedArgs, nil
}

func ArgsToCalldata(abiPath string, method string, args []string) ([]byte, error) {
	abiFile, err := os.ReadFile(abiPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read ABI file: %w", err)
	}
	var contractAbi abi.ABI
	if err := json.Unmarshal(abiFile, &contractAbi); err != nil {
		return nil, fmt.Errorf("failed to parse ABI: %w", err)
	}

	inputs := contractAbi.Constructor.Inputs
	if method != "" {
		inputs = contractAbi.Methods[method].Inputs
	}

	methodArgs, err := parseCallArguments(args, inputs)
	if err != nil {
		return nil, fmt.Errorf("failed to parse method arguments: %w", err)
	}
	calldata, err := contractAbi.Pack(method, methodArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to pack method call: %w", err)
	}
	return calldata, nil
}

func ReadBytecode(filename string, abiPath string, args []string) ([]byte, error) {
	var bytecode []byte
	var err error
	if filename != "" {
		codeHex, err := os.ReadFile(filename)
		if err != nil {
			return nil, fmt.Errorf("failed to read file: %w", err)
		}

		bytecode = hexutil.FromHex(string(codeHex))
		if abiPath != "" {
			calldata, err := ArgsToCalldata(abiPath, "", args)
			if err != nil {
				return nil, fmt.Errorf("failed to handle constructor arguments: %w", err)
			}
			bytecode = append(bytecode, calldata...)
		}
	} else {
		scanner := bufio.NewScanner(os.Stdin)
		input := ""
		for scanner.Scan() {
			input += scanner.Text()
		}
		if err := scanner.Err(); err != nil {
			return nil, fmt.Errorf("failed to read from stdin: %w", err)
		}
		bytecode, err = hex.DecodeString(input)
		if err != nil {
			return nil, fmt.Errorf("failed to decode hex: %w", err)
		}
	}
	return bytecode, nil
}
