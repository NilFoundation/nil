package common

import (
	"bufio"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"strings"

	"github.com/NilFoundation/nil/common/hexutil"
	"github.com/ethereum/go-ethereum/accounts/abi"
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

func parseCallArguments(args []string) []interface{} {
	var parsedArgs []interface{}
	for _, arg := range args {
		if i, ok := new(big.Int).SetString(arg, 10); ok {
			parsedArgs = append(parsedArgs, i)
		} else {
			parsedArgs = append(parsedArgs, arg)
		}
	}
	return parsedArgs
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

	methodArgs := parseCallArguments(args)
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
			calldata, err := ArgsToCalldata(abiPath, "", args[1:])
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
