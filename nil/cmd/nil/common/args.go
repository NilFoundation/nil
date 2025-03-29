package common

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"os"
	"reflect"
	"strconv"
	"strings"

	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/NilFoundation/nil/nil/internal/abi"
	"github.com/NilFoundation/nil/nil/internal/types"
)

func PrepareArgs(contractAbi abi.ABI, calldataOrMethod string, args []string) ([]byte, error) {
	var calldata []byte
	if strings.HasPrefix(calldataOrMethod, "0x") {
		calldata = hexutil.FromHex(calldataOrMethod)
	} else {
		var err error
		calldata, err = ArgsToCalldata(contractAbi, calldataOrMethod, args)
		if err != nil {
			return nil, err
		}
	}
	return calldata, nil
}

func parseCallArgument(arg string, tp abi.Type) (any, error) {
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
	case abi.FixedBytesTy:
		data, err := hexutil.DecodeHex(arg)
		if err != nil {
			return nil, fmt.Errorf("failed to parse bytes argument: %w", err)
		}
		if len(data) != tp.Size {
			return nil, fmt.Errorf("invalid data size: expected %d but got %d", tp.Size, len(data))
		}
		reflect.Copy(val, reflect.ValueOf(data[0:tp.Size]))
	case abi.BytesTy:
		data, err := hexutil.DecodeHex(arg)
		if err != nil {
			return nil, fmt.Errorf("failed to parse bytes argument: %w", err)
		}
		val.SetBytes(data)
	case abi.BoolTy:
		valBool, err := strconv.ParseBool(arg)
		if err != nil {
			return nil, fmt.Errorf("failed to parse bool argument: %w", err)
		}
		val.SetBool(valBool)
	case abi.AddressTy:
		var address types.Address
		if err := address.UnmarshalText([]byte(arg)); err != nil {
			return nil, fmt.Errorf("failed to parse address argument: %w", err)
		}
		val.Set(reflect.ValueOf(address))
	case abi.SliceTy:
		for _, arg := range strings.Split(arg, ",") {
			elem, err := parseCallArgument(arg, *tp.Elem)
			if err != nil {
				return nil, fmt.Errorf("failed to parse slice argument: %w", err)
			}
			val.Set(reflect.Append(val, reflect.ValueOf(elem)))
		}
	default:
		return nil, fmt.Errorf("unsupported argument type: %s", tp.String())
	}
	return val.Interface(), nil
}

func parseCallArguments(args []string, inputs abi.Arguments) ([]any, error) {
	parsedArgs := make([]any, 0, len(args))
	if len(args) != len(inputs) {
		return nil, fmt.Errorf(
			"invalid amount of arguments is provided: expected %d but got %d", len(inputs), len(args))
	}

	for ind, arg := range args {
		val, err := parseCallArgument(arg, inputs[ind].Type)
		if err != nil {
			return nil, fmt.Errorf("failed to parse argument %d: %w", ind, err)
		}
		parsedArgs = append(parsedArgs, val)
	}
	return parsedArgs, nil
}

func ArgsToCalldata(contractAbi abi.ABI, method string, args []string) ([]byte, error) {
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

type ArgValue struct {
	Type  string `json:"type"`
	Value any    `json:"value"`
}

type NamedArgValues struct {
	Name      string      `json:"name"`
	ArgValues []*ArgValue `json:"values"`
}

func ReadAbiFromFile(abiPath string) (abi.ABI, error) {
	abiFile, err := os.ReadFile(abiPath)
	if err != nil {
		return abi.ABI{}, err
	}

	return abi.JSON(bytes.NewReader(abiFile))
}

func FetchAbiFromCometa(addr types.Address) (abi.ABI, error) {
	client := GetCometaRpcClient()
	if client == nil {
		return abi.ABI{}, errors.New("cometa client is not initialized")
	}
	res, err := client.GetAbi(addr)
	if err != nil {
		return abi.ABI{}, fmt.Errorf("failed to fetch contract from cometa: %w", err)
	}
	return res, nil
}

func DecodeLogs(abi abi.ABI, logs []*types.Log) ([]*NamedArgValues, error) {
	decodedLogs := make([]*NamedArgValues, 0, len(logs))

	for _, log := range logs {
		if len(log.Topics) == 0 {
			continue
		}

		event, err := abi.EventByID(log.Topics[0])
		if err != nil {
			return nil, fmt.Errorf("failed to find event by topic: %w", err)
		}
		if event == nil {
			continue
		}

		if len(log.Data) == 0 {
			continue
		}

		obj, err := abi.Unpack(event.Name, log.Data)
		if err != nil {
			return nil, fmt.Errorf("failed to unpack log %q data: %w", event.Name, err)
		}

		log := make([]*ArgValue, len(event.Inputs))
		for i, arg := range event.Inputs {
			log[i] = &ArgValue{
				Type:  arg.Type.String(),
				Value: obj[i],
			}
		}
		decodedLogs = append(decodedLogs, &NamedArgValues{
			Name:      event.Name,
			ArgValues: log,
		})
	}

	return decodedLogs, nil
}

func CalldataToArgs(abi abi.ABI, method string, data []byte) ([]*ArgValue, error) {
	obj, err := abi.Unpack(method, data)
	if err != nil {
		return nil, err
	}

	results := make([]*ArgValue, len(abi.Methods[method].Outputs))
	for i, output := range abi.Methods[method].Outputs {
		results[i] = &ArgValue{
			Type:  output.Type.String(),
			Value: obj[i],
		}
	}
	return results, nil
}

func ReadBytecode(filename string, abiPath string, args []string) (types.Code, error) {
	var bytecode []byte
	var err error
	location := filename
	if filename != "" {
		codeHex, err := os.ReadFile(filename)
		if err != nil {
			return nil, fmt.Errorf("failed to read file: %w", err)
		}

		bytecode = hexutil.FromHex(string(codeHex))
		if abiPath != "" {
			abi, err := ReadAbiFromFile(abiPath)
			if err != nil {
				return nil, fmt.Errorf("failed to read abi: %w", err)
			}

			calldata, err := ArgsToCalldata(abi, "", args)
			if err != nil {
				return nil, fmt.Errorf("failed to handle constructor arguments: %w", err)
			}
			bytecode = append(bytecode, calldata...)
		}
	} else {
		location = "standard input"
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
	if len(bytecode) == 0 {
		return nil, fmt.Errorf("read empty bytecode from %s", location)
	}
	return bytecode, nil
}

func ParseTokens(params []string) ([]types.TokenBalance, error) {
	tokens := make([]types.TokenBalance, 0, len(params))
	for _, token := range params {
		tokAndBalance := strings.Split(token, "=")
		if len(tokAndBalance) != 2 {
			return nil, fmt.Errorf("invalid token format: %s, expected <tokenId>=<balance>", token)
		}
		// Not using Hash.Set because want to be able to parse tokenId without leading zeros
		tokenBytes, err := hexutil.DecodeHex(tokAndBalance[0])
		if err != nil {
			return nil, fmt.Errorf("invalid token id %s, can't parse hex: %w", tokAndBalance[0], err)
		}
		tokenId := types.TokenId(types.BytesToAddress(tokenBytes))
		var balance types.Value
		if err := balance.Set(tokAndBalance[1]); err != nil {
			return nil, fmt.Errorf("invalid balance %s: %w", tokAndBalance[1], err)
		}
		tokens = append(tokens, types.TokenBalance{Token: tokenId, Balance: balance})
	}
	return tokens, nil
}
