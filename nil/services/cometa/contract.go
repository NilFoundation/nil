package cometa

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/NilFoundation/nil/nil/internal/abi"
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

type Location struct {
	FileName string `json:"fileName"` // FileName is the name of the source file.
	Position uint   `json:"position"` // Position is the position in chars.
	Length   uint   `json:"length"`   // Length is the length of the code segment.
}

type LineLocation struct {
	FileName string // FileName is the name of the source file.
	Line     uint   // Position is the position in chars.
	Column   uint   // Column is the position in chars.
	Length   uint   // Length is the length of the code segment.
}

func NewContractFromData(data *ContractData) (*Contract, error) {
	c := &Contract{
		Data: data,
	}
	err := json.Unmarshal([]byte(data.Metadata), &c.Metadata)
	if err != nil {
		return nil, err
	}
	d, err := json.Marshal(data.Abi)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal abi: %w", err)
	}
	abi, err := abi.JSON(strings.NewReader(string(d)))
	if err != nil {
		return nil, fmt.Errorf("failed to parse abi: %w", err)
	}
	c.abi = &abi

	return c, nil
}

func (l *Location) String() string {
	return fmt.Sprintf("%s:%d", l.FileName, l.Position)
}

func (l *LineLocation) String() string {
	return fmt.Sprintf("%s:%d:[%d-%d]", l.FileName, l.Line, l.Column, l.Column+l.Length)
}

// GetLocation returns the location of the given program counter in the source code.
func (c *Contract) GetLocation(pc uint) (*Location, error) {
	if err := c.decodeSourceMap(); err != nil {
		return nil, err
	}

	inst := c.bytecode2inst[pc]
	loc := &c.sourceMap[inst]

	sourceFile := c.Data.SourceFilesList[loc.FileNum]

	return &Location{
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

func (c *Contract) GetLineLocation(pc uint) (*LineLocation, error) {
	loc, err := c.GetLocation(pc)
	if err != nil {
		return nil, fmt.Errorf("failed to get location: %w", err)
	}
	return c.getLineLocation(loc)
}

func (c *Contract) getLineLocation(loc *Location) (*LineLocation, error) {
	source, ok := c.Data.SourceCode[loc.FileName]
	if !ok {
		return nil, fmt.Errorf("source file not found: '%s'", loc.FileName)
	}
	lineLoc := &LineLocation{FileName: loc.FileName, Column: 1, Line: 1, Length: loc.Length}
	pos := uint(0)
	for _, ch := range source {
		if pos >= loc.Position {
			break
		}
		if ch == '\n' {
			lineLoc.Line += 1
			lineLoc.Column = 1
		} else {
			lineLoc.Column += 1
		}
		pos += 1
	}
	return lineLoc, nil
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
	for signature, funcId := range c.Data.CompilerOutput.Evm.MethodIdentifiers {
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
	for i, arg := range args {
		if i > 0 {
			res += fmt.Sprintf(", %v", arg)
		} else {
			res += fmt.Sprintf("%v", arg)
		}
	}
	res += ")"

	return res, nil
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
		instIndex += 1
	}

	return nil
}
