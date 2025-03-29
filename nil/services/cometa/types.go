package cometa

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var ErrAbiNotFound = errors.New("abi not found")

// CompilerJsonInput represents the input structure for the solidity compiler.
type CompilerJsonInput struct {
	Language string             `json:"language"`
	Sources  map[string]*Source `json:"sources"`
	Settings CompilerSettings   `json:"settings"`
}

type Source struct {
	Keccak256 string   `json:"keccak256,omitempty"`
	Urls      []string `json:"urls,omitempty"`
	Content   string   `json:"content,omitempty"`
}

type CompilerSettings struct {
	StopAfter       string           `json:"stopAfter,omitempty"`
	Remappings      []string         `json:"remappings,omitempty"`
	Optimizer       Optimizer        `json:"optimizer"`
	EvmVersion      string           `json:"evmVersion"`
	ViaIR           bool             `json:"viaIR,omitempty"` //nolint:tagliatelle
	Debug           any              `json:"debug,omitempty"`
	Metadata        SettingsMetadata `json:"metadata"`
	OutputSelection map[string]any   `json:"outputSelection,omitempty"`
}

type Optimizer struct {
	Enabled bool `json:"enabled"`
	Runs    int  `json:"runs"`
	Details any  `json:"details,omitempty"`
}

type SettingsMetadata struct {
	AppendCBOR        bool   `json:"appendCBOR"` //nolint:tagliatelle
	UseLiteralContent bool   `json:"useLiteralContent"`
	BytecodeHash      string `json:"bytecodeHash,omitempty"`
}

// CompilerJsonOutput represents the output structure of the solidity compiler.
type CompilerJsonOutput struct {
	Errors    []CompilerOutputError                        `json:"errors"`
	Sources   map[string]CompilerOutputSource              `json:"sources"`
	Contracts map[string]map[string]CompilerOutputContract `json:"contracts"`
}

type CompilerOutputError struct {
	SourceLocation   SourceLocation `json:"sourceLocation"`
	Type             string         `json:"type"`
	Component        string         `json:"component"`
	Severity         string         `json:"severity"`
	Message          string         `json:"message"`
	FormattedMessage string         `json:"formattedMessage"`
}

type SourceLocation struct {
	File  string `json:"file"`
	Start int    `json:"start"`
	End   int    `json:"end"`
}

type CompilerOutputSource struct {
	Id        int `json:"id"`
	Ast       any `json:"ast"`
	LegacyAst any `json:"legacyAst"`
}

type CompilerOutputContract struct {
	Abi            []any     `json:"abi"`
	Metadata       string    `json:"metadata,omitempty"`
	Userdoc        any       `json:"userdoc,omitempty"`
	Devdoc         any       `json:"devdoc,omitempty"`
	Ir             string    `json:"ir,omitempty"`
	IrAst          any       `json:"irAst,omitempty"`
	IrOptimized    string    `json:"irOptimized,omitempty"`
	IrOptimizedAst any       `json:"irOptimizedAst,omitempty"`
	StorageLayout  any       `json:"storageLayout,omitempty"`
	Evm            EvmOutput `json:"evm"`
}

type EvmOutput struct {
	Assembly          string            `json:"assembly,omitempty"`
	LegacyAssembly    any               `json:"legacyAssembly,omitempty"`
	Bytecode          CompilerOutputEvm `json:"bytecode"`
	DeployedBytecode  CompilerOutputEvm `json:"deployedBytecode"`
	MethodIdentifiers map[string]string `json:"methodIdentifiers,omitempty"`
	GasEstimates      any               `json:"gasEstimates,omitempty"`
}

type CompilerOutputEvm struct {
	Object              string            `json:"object,omitempty"`
	Opcodes             string            `json:"opcodes,omitempty"`
	SourceMap           string            `json:"sourceMap,omitempty"`
	LinkReferences      any               `json:"linkReferences,omitempty"`
	ImmutableReferences any               `json:"immutableReferences,omitempty"`
	FunctionDebugData   FunctionDebugData `json:"functionDebugData"`
	GeneratedSources    []GeneratedSource `json:"generatedSources,omitempty"`
}

type GeneratedSource struct {
	Contents string `json:"contents"`
	Id       int    `json:"id"`
	Language string `json:"language"`
	Name     string `json:"name"`
}

type Metadata struct {
	Compiler CompilerVersion   `json:"compiler"`
	Language string            `json:"language,omitempty"`
	Output   any               `json:"output,omitempty"`
	Settings MetadataSettings  `json:"settings"`
	Sources  map[string]Source `json:"sources,omitempty"`
	Version  int               `json:"version"`
}

type CompilerVersion struct {
	Keccak256 string `json:"keccak256,omitempty"`
	Version   string `json:"version"`
}

type MetadataSettings struct {
	CompilationTarget map[string]string `json:"compilationTarget,omitempty"`
	EvmVersion        string            `json:"evmVersion,omitempty"`
	Libraries         any               `json:"libraries,omitempty"`
	Optimizer         Optimizer         `json:"optimizer"`
	OutputSelection   map[string]any    `json:"outputSelection,omitempty"`
}

// CompilerTask is the input for the service. It contains all information for compilation and deployment.
type CompilerTask struct {
	ContractName    string             `json:"contractName,omitempty"`
	CompilerVersion string             `json:"compilerVersion,omitempty"`
	BasePath        string             `json:"basePath,omitempty"`
	Sources         map[string]*Source `json:"sources,omitempty"`
	Settings        Settings           `json:"settings"`
	// SolcStandardJson is used to pass raw input to the compiler. If it is specified, it will be directly passed to
	// solc compiler via `--standard-json` argument.
	SolcStandardJson *CompilerJsonInput `json:"solcStandardJson,omitempty"`

	// isNormalized is used to check if the input has been already normalized.
	isNormalized bool
}

type Settings struct {
	Optimizer    Optimizer `json:"optimizer"`
	EvmVersion   string    `json:"evmVersion"`
	AppendCBOR   bool      `json:"appendCBOR"` //nolint:tagliatelle
	BytecodeHash string    `json:"bytecodeHash"`
}

type FunctionDebugItem struct {
	Name           string `json:"name,omitempty"`
	EntryPoint     int    `json:"entryPoint"` // EntryPoint is the offset in the bytecode where the function starts.
	Id             int    `json:"id"`
	ParameterSlots int    `json:"parameterSlots"`
	ReturnSlots    int    `json:"returnSlots"`
}

type FunctionDebugData map[string]FunctionDebugItem

// Normalize fills in the content of the sources that have no content but have urls.
func (t *CompilerTask) Normalize(basePath string) error {
	if t.isNormalized {
		return nil
	}
	for _, source := range t.Sources {
		if source.Content != "" {
			continue
		}
		if len(source.Urls) != 1 {
			return errors.New("source has no content and has invalid number of urls")
		}
		fname := source.Urls[0]
		if !filepath.IsAbs(fname) {
			fname = filepath.Join(basePath, fname)
		}
		data, err := os.ReadFile(fname)
		if err != nil {
			return fmt.Errorf("failed to read source file: %w", err)
		}
		source.Content = string(data)
		source.Urls = nil
	}
	t.isNormalized = true
	return nil
}

func (t *CompilerTask) CheckResolved() error {
	for _, source := range t.Sources {
		if source.Content == "" {
			return fmt.Errorf("source %s has no content", source)
		}
	}
	return nil
}

// ToCompilerJsonInput converts CompilerTask to CompilerJsonInput, which can be consumed by the compiler.
func (t *CompilerTask) ToCompilerJsonInput() (*CompilerJsonInput, error) {
	if err := t.CheckResolved(); err != nil {
		return nil, err
	}
	res := &CompilerJsonInput{
		Language: "Solidity",
	}
	res.Sources = t.Sources
	res.Settings.Optimizer = t.Settings.Optimizer
	res.Settings.EvmVersion = t.Settings.EvmVersion
	res.Settings.Metadata.BytecodeHash = t.Settings.BytecodeHash
	res.Settings.Metadata.AppendCBOR = t.Settings.AppendCBOR
	parts := strings.Split(t.ContractName, ":")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid contract name: %s, required format: <file>:<contract>", t.ContractName)
	}
	res.Settings.OutputSelection = map[string]any{
		parts[0]: map[string]any{
			parts[1]: []string{
				"abi",
				"metadata",
				"evm.bytecode.object",
				"evm.deployedBytecode.object",
				"evm.deployedBytecode.sourceMap",
				"evm.deployedBytecode.generatedSources",
				"evm.deployedBytecode.functionDebugData",
				"evm.methodIdentifiers",
			},
		},
	}

	return res, nil
}

func (c *CompilerOutputContract) Validate() error {
	if len(c.Abi) == 0 {
		return errors.New("abi is required")
	}
	if len(c.Metadata) == 0 {
		return errors.New("metadata is required")
	}
	if c.Evm.Bytecode.Object == "" {
		return errors.New("evm.bytecode.object is required")
	}
	if c.Evm.DeployedBytecode.Object == "" {
		return errors.New("evm.deployedBytecode.object is required")
	}
	if c.Evm.DeployedBytecode.SourceMap == "" {
		return errors.New("evm.deployedBytecode.sourceMap is required")
	}
	if len(c.Evm.MethodIdentifiers) == 0 {
		return errors.New("evm.methodIdentifiers is required")
	}

	return nil
}
