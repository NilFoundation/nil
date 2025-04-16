package console

import (
	"encoding/binary"
	"errors"
	"fmt"
	"strings"

	"github.com/NilFoundation/nil/nil/internal/types"
)

const wordSize = 32

func ProcessLog(input []byte) (string, error) {
	if len(input) < 4 {
		return "", errors.New("input data size is less than 4 bytes")
	}
	funcId := binary.BigEndian.Uint32(input[:4])
	input = input[4:]

	params, ok := LogSignatures[funcId]
	if !ok {
		return "", fmt.Errorf("unknown log signature: %x", funcId)
	}
	if params[0] != StringTy {
		return "", errors.New("first parameter must be a format string")
	}
	format := readString(input, 0)
	sb := strings.Builder{}
	pos := wordSize
	paramIndex := 1
	i := 0
	for i < len(format)-1 {
		processed := false
		if format[i] == '%' {
			switch format[i+1] {
			case '%':
				sb.WriteString(string(format[i]))
				i += 2
				processed = true
			case '_', 'x':
				if paramIndex >= len(params) {
					return "", errors.New("not enough parameters in input data")
				}
				sb.WriteString(readParam(input, pos, params[paramIndex], format[i+1] == 'x'))
				pos += wordSize
				paramIndex++
				i += 2
				processed = true
			}
		}
		if !processed {
			sb.WriteString(string(format[i]))
			i++
		}
	}
	if i == len(format)-1 {
		sb.WriteString(string(format[len(format)-1]))
	}
	return sb.String(), nil
}

func readParam(input []byte, pos int, paramType ParamType, hex bool) string {
	switch paramType {
	case Uint256Ty:
		val := types.NewUint256FromBytes(input[pos : pos+wordSize])
		if hex {
			return val.Int().Hex()
		}
		return val.String()
	case AddressTy:
		val := types.BytesToAddress(input[pos : pos+wordSize])
		return fmt.Sprintf("0x%x", val)
	case BoolTy:
		if input[pos+wordSize-1] == 0 {
			return "false"
		}
		return "true"
	case StringTy:
		return readString(input, pos)
	case NoneTy:
		return "<error: none type>"
	}
	return "<error: unknown type>"
}

func readString(input []byte, pos int) string {
	start := binary.BigEndian.Uint32(input[pos+wordSize-4 : pos+wordSize])
	length := binary.BigEndian.Uint32(input[start+wordSize-4 : start+wordSize])
	str := string(input[start+wordSize : start+wordSize+length])
	return str
}
