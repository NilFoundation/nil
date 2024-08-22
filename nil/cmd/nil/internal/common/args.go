package common

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"reflect"
	"strconv"
	"strings"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/NilFoundation/nil/nil/internal/abi"
	"github.com/NilFoundation/nil/nil/internal/types"
	eth_common "github.com/ethereum/go-ethereum/common"
)

func PrepareArgs(abiPath string, calldataOrMethod string, args []string) ([]byte, error) {
	var calldata []byte
	if strings.HasPrefix(calldataOrMethod, "0x") && abiPath == "" {
		calldata = hexutil.FromHex(calldataOrMethod)
	} else {
		var err error
		calldata, err = ArgsToCalldata(abiPath, calldataOrMethod, args)
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
		var address eth_common.Address
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
		return nil, fmt.Errorf("invalid amout of arguments is provided: expected %d but got %d", len(inputs), len(args))
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

type ArgValue struct {
	Type  abi.Type
	Value any
}

func CalldataToArgs(abiPath string, method string, data []byte) ([]*ArgValue, error) {
	abiFile, err := os.ReadFile(abiPath)
	if err != nil {
		return nil, err
	}

	abi, err := abi.JSON(bytes.NewReader(abiFile))
	if err != nil {
		return nil, err
	}

	obj, err := abi.Unpack(method, data)
	if err != nil {
		return nil, err
	}

	results := make([]*ArgValue, len(abi.Methods[method].Outputs))
	for i, output := range abi.Methods[method].Outputs {
		results[i] = &ArgValue{
			Type:  output.Type,
			Value: obj[i],
		}
	}
	return results, nil
}

func ReadBytecode(filename string, abiPath string, args []string) (types.Code, error) {
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

func ParseCurrencies(params []string) ([]types.CurrencyBalance, error) {
	currencies := make([]types.CurrencyBalance, 0, len(params))
	for _, currency := range params {
		curAndBalance := strings.Split(currency, "=")
		if len(curAndBalance) != 2 {
			return nil, fmt.Errorf("invalid currency format: %s, expected <currencyId>=<balance>", currency)
		}
		// Not using Hash.Set because want to be able to parse currencyId without leading zeros
		currencyBytes, err := hexutil.DecodeHex(curAndBalance[0])
		if err != nil {
			return nil, fmt.Errorf("invalid currency id %s, can't parse hex: %w", curAndBalance[0], err)
		}
		currencyId := types.CurrencyId(common.BytesToHash(currencyBytes))
		var balance types.Value
		if err := balance.Set(curAndBalance[1]); err != nil {
			return nil, fmt.Errorf("invalid balance %s: %w", curAndBalance[1], err)
		}
		currencies = append(currencies, types.CurrencyBalance{Currency: currencyId, Balance: balance})
	}
	return currencies, nil
}
