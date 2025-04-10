package cometa

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/NilFoundation/nil/nil/internal/abi"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
)

type Contract struct {
	Data      *ContractData     // Data contains the contract data which is stored in db.
	Metadata  *Metadata         // Metadata contains the contract metadata retrieved after compilation.
	sourceMap []DecodedLocation // sourceMap is a decoded source map.
	// bytecode2inst maps bytecode offset to instruction index. It is needed because some instructions are longer than
	// 1 byte.
	bytecode2inst []int

	abi *abi.ABI
}

type DecodedLocation struct {
	FileNum  int // FileNum is the index of the source file.
	StartPos int // StartPos is the starting position in the source file.
	Length   int // Length is the length of the code segment.
	Jump     int // Jump indicates the jump type.
}

type LocationRaw struct {
	FileName string `json:"fileName"` // FileName is the name of the source file.
	Position uint   `json:"position"` // Position is the position in chars.
	Length   uint   `json:"length"`   // Length is the length of the code segment.
}

type Location struct {
	FileName string `json:"fileName"` // FileName is the name of the source file.
	Function string `json:"function"` // Function is the name of the function.
	Line     uint   `json:"line"`     // Position is the position in chars.
	Column   uint   `json:"column"`   // Column is the position in chars.
	Length   uint   `json:"length"`   // Length is the length of the code segment.
}

func NewContractFromData(data *ContractData) (*Contract, error) {
	c := &Contract{
		Data: data,
	}
	err := json.Unmarshal([]byte(data.Metadata), &c.Metadata)
	if err != nil {
		return nil, err
	}
	abi, err := abi.JSON(strings.NewReader(data.Abi))
	if err != nil {
		return nil, fmt.Errorf("failed to parse abi: %w", err)
	}
	c.abi = &abi

	return c, nil
}

func (l *LocationRaw) String() string {
	return fmt.Sprintf("%s:%d", l.FileName, l.Position)
}

func (l *Location) String() string {
	return fmt.Sprintf("%s:%d, function: %s", l.FileName, l.Line, l.Function)
}

func (c *Contract) GetMethodSignatureById(methodId string) string {
	for signature, id := range c.Data.MethodIdentifiers {
		if id == methodId {
			return signature
		}
	}
	return ""
}

// GetLocationRaw returns the location of the given program counter in the source code.
func (c *Contract) GetLocationRaw(pc uint) (*LocationRaw, error) {
	if err := c.decodeSourceMap(); err != nil {
		return nil, err
	}

	inst := c.bytecode2inst[pc]
	loc := &c.sourceMap[inst]

	sourceFile := c.Data.SourceFilesList[loc.FileNum]

	return &LocationRaw{
		FileName: sourceFile,
		Position: uint(loc.StartPos),
		Length:   uint(loc.Length),
	}, nil
}

func (c *Contract) ShortName() string {
	parts := strings.Split(c.Data.Name, ":")
	if len(parts) > 1 {
		return parts[1]
	}
	return c.Data.Name
}

func (c *Contract) GetLocation(pc uint) (*Location, error) {
	loc, err := c.GetLocationRaw(pc)
	if err != nil {
		return nil, fmt.Errorf("failed to get location: %w", err)
	}
	return c.getLocation(pc, loc)
}

func (c *Contract) getLocation(pc uint, locRaw *LocationRaw) (*Location, error) {
	source, ok := c.Data.SourceCode[locRaw.FileName]
	if !ok {
		return nil, fmt.Errorf("source file not found: '%s'", locRaw.FileName)
	}
	loc := &Location{FileName: locRaw.FileName, Column: 1, Line: 1, Length: locRaw.Length}
	pos := uint(0)
	for _, ch := range source {
		if pos >= locRaw.Position {
			break
		}
		if ch == '\n' {
			loc.Line++
			loc.Column = 1
		} else {
			loc.Column++
		}
		pos++
	}

	funcIndex := sort.Search(len(c.Data.FunctionDebugData), func(i int) bool {
		return c.Data.FunctionDebugData[i].EntryPoint >= int(pc)
	})
	if funcIndex >= 1 && funcIndex < len(c.Data.FunctionDebugData) {
		loc.Function = c.Data.FunctionDebugData[funcIndex-1].Name
	} else if funcIndex == 0 {
		// In some cases, functionDebugData doesn't contain the entry point for the function selector.
		loc.Function = "#function_selector"
	}

	return loc, nil
}

func (c *Contract) DecodeCallData(calldata []byte) (string, error) {
	if len(calldata) == 0 {
		return "", nil
	}
	if len(calldata)%2 != 0 {
		return "", fmt.Errorf("invalid calldata length: %d", len(calldata))
	}

	hexFuncId := hexutil.EncodeNo0x(calldata[:4])
	methodSignature := ""
	for signature, funcId := range c.Data.MethodIdentifiers {
		if hexFuncId == funcId {
			methodSignature = signature
			break
		}
	}
	if methodSignature == "" {
		return "", fmt.Errorf("method not found for id=%s", hexFuncId)
	}
	parts := strings.Split(methodSignature, "(")
	methodName := parts[0]
	method, ok := c.abi.Methods[methodName]
	if !ok {
		return "", fmt.Errorf("method not found in ABI: %s", methodName)
	}
	if len(calldata) < 4 {
		return "", fmt.Errorf("too short calldata: %d", len(calldata))
	}
	args, err := method.Inputs.Unpack(calldata[4:])
	if err != nil {
		return "", fmt.Errorf("failed to unpack arguments: %w", err)
	}
	res := methodName + "("
	adjustArg := func(arg any) string {
		switch v := arg.(type) {
		case []byte:
			return hexutil.Encode(v)
		default:
			return fmt.Sprintf("%v", v)
		}
	}
	for i, arg := range args {
		if i > 0 {
			res += fmt.Sprintf(", %v", adjustArg(arg))
		} else {
			res += adjustArg(arg)
		}
	}
	res += ")"

	return res, nil
}

func (c *Contract) DecodeLog(log *jsonrpc.RPCLog) (string, error) {
	var res strings.Builder
	if len(log.Topics) == 0 {
		return "", nil
	}

	event, err := c.abi.EventByID(log.Topics[0])
	if err != nil {
		return "", fmt.Errorf("failed to find event by topic: %w", err)
	}
	if event == nil {
		return "", nil
	}
	if len(log.Data) == 0 {
		return "", nil
	}

	obj, err := c.abi.Unpack(event.Name, log.Data)
	if err != nil {
		return "", fmt.Errorf("failed to unpack log %q data: %w", event.Name, err)
	}

	res.WriteString(event.Name)
	res.WriteString(": ")

	values, err := json.Marshal(obj)
	if err != nil {
		return "", fmt.Errorf("failed to marshal log %q data: %w", event.Name, err)
	}
	res.Write(values)
	return res.String(), nil
}

func (c *Contract) GetSourceLines(sourceFile string) ([]string, error) {
	source, ok := c.Data.SourceCode[sourceFile]
	if !ok {
		return nil, fmt.Errorf("source file not found: %s", sourceFile)
	}
	return strings.Split(source, "\n"), nil
}

// decodeSourceMap decodes the source map string into array of DecodedLocation records. After that, for each bytecode
// instruction there is a corresponding DecodedLocation record.
func (c *Contract) decodeSourceMap() error {
	if len(c.bytecode2inst) != 0 {
		return nil
	}
	items := strings.Split(c.Data.SourceMap, ";")
	c.sourceMap = make([]DecodedLocation, 0, len(items))
	prevItem := &DecodedLocation{}
	for i, item := range items {
		if len(item) == 0 {
			c.sourceMap = append(c.sourceMap, *prevItem)
			prevItem = &c.sourceMap[i]
			continue
		}
		parts := strings.Split(item, ":")
		c.sourceMap = append(c.sourceMap, *prevItem)
		prevItem = &c.sourceMap[i]

		if len(parts) < 1 {
			continue
		}
		if len(parts[0]) > 0 {
			val, err := strconv.ParseInt(parts[0], 10, 64)
			if err != nil {
				return err
			}
			c.sourceMap[i].StartPos = int(val)
		}

		if len(parts) < 2 {
			continue
		}
		if len(parts[1]) > 0 {
			val, err := strconv.ParseInt(parts[1], 10, 64)
			if err != nil {
				return err
			}
			c.sourceMap[i].Length = int(val)
		}

		if len(parts) < 3 {
			continue
		}
		if len(parts[2]) > 0 {
			val, err := strconv.ParseInt(parts[2], 10, 64)
			if err != nil {
				return err
			}
			c.sourceMap[i].FileNum = int(val)
		}
	}

	c.bytecode2inst = make([]int, len(c.Data.Code))

	instructionLength := func(opcode uint8) int {
		if opcode >= 0x60 && opcode <= 0x7f {
			return int(opcode-0x5f) + 1
		}
		return 1
	}

	instIndex := 0
	for bytecodeIndex := 0; bytecodeIndex < len(c.Data.Code); {
		opcode := c.Data.Code[bytecodeIndex]
		length := instructionLength(opcode)
		if bytecodeIndex+length > len(c.Data.Code) {
			// We reached bytecode's metadata.
			break
		}
		for i := range length {
			c.bytecode2inst[bytecodeIndex+i] = instIndex
		}
		bytecodeIndex += length
		instIndex++
	}

	return nil
}
