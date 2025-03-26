package cometa

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/fabelx/go-solc-select/pkg/config"
	"github.com/fabelx/go-solc-select/pkg/installer"
	"github.com/fabelx/go-solc-select/pkg/versions"
)

const (
	GeneratedSourceFileName = "#utility.yul"
)

type ContractData struct {
	// Name holds contract name.
	Name string `json:"name,omitempty"`

	// Description holds optional description of the contract.
	Description string `json:"description,omitempty"`

	// Abi holds contract ABI in json format.
	Abi string `json:"abi,omitempty"`

	// SourceCode holds source code content for each file: {name -> content}
	SourceCode map[string]string `json:"sourceCode,omitempty"`

	// SourceMap holds mappings between bytecode and source code.
	// See https://docs.soliditylang.org/en/latest/internals/source_mappings.html
	SourceMap string `json:"sourceMap,omitempty"`

	// Metadata holds metadata in JSON format, directly copied from the compiler output.
	Metadata string `json:"metadata,omitempty"`

	// InitCode holds bytecode for contract deployment.
	InitCode []byte `json:"initCode,omitempty"`

	// Code holds runtime bytecode which is stored in blockchain.
	Code []byte `json:"code,omitempty"`

	// SourceFilesList holds a list of source files, ordered as referred to in debug entities like sourceMap.
	// The file ID in sourceMap corresponds to the index in this array.
	SourceFilesList []string `json:"sourceFilesList,omitempty"`

	// FunctionDebugData holds a list of functions locations, where location is an entry bytecode offset.
	// The list is sorted by entry bytecode offset.
	FunctionDebugData []FunctionDebugItem `json:"functionDebugData,omitempty"`

	// MethodIdentifiers holds a map of method identifiers: {signature -> methodId}. E.g. "test(uint256)": "29e99f07"
	MethodIdentifiers map[string]string `json:"methodIdentifiers,omitempty"`
}

func NewCompilerTask(inputJson string) (*CompilerTask, error) {
	var task CompilerTask
	if err := json.Unmarshal([]byte(inputJson), &task); err != nil {
		return nil, err
	}
	return &task, nil
}

func CompileJson(inputJson string) (*ContractData, error) {
	input, err := NewCompilerTask(inputJson)
	if err != nil {
		return nil, fmt.Errorf("failed to read input json: %w", err)
	}
	return Compile(input)
}

func Compile(input *CompilerTask) (*ContractData, error) {
	logger.Info().Msg("Start contract compilation...")
	dir, err := os.MkdirTemp("/tmp", "compilation_")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(dir)

	solc, err := findCompiler(input.CompilerVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to find compiler: %w", err)
	}

	compilerInput := input.SolcStandardJson
	if input.SolcStandardJson == nil {
		compilerInput, err = input.ToCompilerJsonInput()
		if err != nil {
			return nil, fmt.Errorf("failed to convert input to compiler input: %w", err)
		}
	}

	compilerInputData, err := json.MarshalIndent(compilerInput, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal compiler input: %w", err)
	}

	inputFile := dir + "/input.json"
	if err = os.WriteFile(inputFile, compilerInputData, 0o600); err != nil {
		return nil, fmt.Errorf("failed to write input file: %w", err)
	}

	args := []string{"--standard-json", inputFile, "--pretty-json"}
	cmd := exec.Command(solc, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Error().Msgf("Compilation failed:\n%s\n", output)
		return nil, err
	}

	var outputJson CompilerJsonOutput
	if err := json.Unmarshal(output, &outputJson); err != nil {
		logger.Error().Err(err).Msg("Failed to unmarshal json")
		return nil, err
	}
	if len(outputJson.Errors) != 0 {
		for _, e := range outputJson.Errors {
			if e.Severity == "error" {
				errMsg, err := json.MarshalIndent(outputJson.Errors, "", "  ")
				if err != nil {
					errMsg = []byte("failed to marshal errors: " + err.Error())
				}
				logger.Error().Msgf("Compilation failed:\n%s\n", errMsg)
				return nil, errors.New(string(errMsg))
			}
		}
	}

	contractData, err := CreateContractData(input.ContractName, compilerInput, &outputJson)
	if err != nil {
		return nil, fmt.Errorf("failed to load contract info: %w", err)
	}
	contractData.Name = input.ContractName

	return contractData, nil
}

func CreateContractData(
	contractFullName string,
	input *CompilerJsonInput,
	outputJson *CompilerJsonOutput,
) (*ContractData, error) {
	contractData := &ContractData{}

	parts := strings.Split(contractFullName, ":")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid contract name: %s, required format: <file>:<contract>", contractFullName)
	}
	fileName := parts[0]
	contractName := parts[1]

	numContracts := len(input.Sources)

	var contractDescr *CompilerOutputContract
	for file, v := range outputJson.Contracts {
		if file != fileName {
			continue
		}
		for c, contract := range v {
			if c == contractName {
				contractDescr = &contract
				break
			}
		}
		if contractDescr != nil {
			break
		}
	}
	if contractDescr == nil {
		return nil, errors.New("contract not found in compilation output")
	}
	if err := contractDescr.Validate(); err != nil {
		return nil, fmt.Errorf("compilation output validation failed: %w", err)
	}

	generatedSourcesExist := false
	if len(contractDescr.Evm.DeployedBytecode.GeneratedSources) != 0 {
		numContracts++
		generatedSourcesExist = true
	}

	contractData.SourceFilesList = make([]string, numContracts)
	contractData.SourceCode = make(map[string]string)

	for k, v := range input.Sources {
		if len(v.Content) != 0 {
			contractData.SourceCode[k] = v.Content
		} else {
			for _, f := range v.Urls {
				content, err := os.ReadFile(f)
				if err != nil {
					return nil, fmt.Errorf("failed to read source file %s: %w", f, err)
				}
				contractData.SourceCode[k] = string(content)
			}
		}
	}

	for k, v := range outputJson.Sources {
		contractData.SourceFilesList[v.Id] = k
	}

	if generatedSourcesExist {
		if len(contractData.SourceFilesList[numContracts-1]) != 0 {
			return nil, errors.New("last id must be empty")
		}
		contractData.SourceFilesList[len(contractData.SourceFilesList)-1] = GeneratedSourceFileName
		contractData.SourceCode[GeneratedSourceFileName] = //
			contractDescr.Evm.DeployedBytecode.GeneratedSources[0].Contents
	}

	contractData.SourceMap = contractDescr.Evm.DeployedBytecode.SourceMap
	if len(contractData.SourceMap) == 0 {
		return nil, errors.New("source map not found")
	}

	for k, v := range contractDescr.Evm.DeployedBytecode.FunctionDebugData {
		v.Name = k
		contractData.FunctionDebugData = append(contractData.FunctionDebugData, v)
	}
	sort.Slice(contractData.FunctionDebugData, func(i, j int) bool {
		return contractData.FunctionDebugData[i].EntryPoint < contractData.FunctionDebugData[j].EntryPoint
	})
	if len(contractData.FunctionDebugData) > 0 {
		for i := range contractData.FunctionDebugData {
			if contractData.FunctionDebugData[i].EntryPoint == 0 {
				contractData.FunctionDebugData[i].Name = "#function_selector"
			} else {
				break
			}
		}
	}
	contractData.Metadata = contractDescr.Metadata
	contractData.Code = hexutil.MustDecode(contractDescr.Evm.DeployedBytecode.Object)
	contractData.InitCode = hexutil.MustDecode(contractDescr.Evm.Bytecode.Object)
	abiJson, err := json.Marshal(contractDescr.Abi)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal abi: %w", err)
	}
	contractData.Abi = string(abiJson)
	contractData.MethodIdentifiers = contractDescr.Evm.MethodIdentifiers

	return contractData, nil
}

func findCompiler(version string) (string, error) {
	installed := versions.GetInstalled()
	_, ok := installed[version]
	if !ok {
		if err := installer.InstallSolc(version); err != nil {
			return "", fmt.Errorf("failed to install compiler %s: %w", version, err)
		}
	}
	solc, ok := versions.GetInstalled()[version]
	if !ok {
		return "", fmt.Errorf("failed to find compiler %s", version)
	}
	solc = "solc-" + solc

	fileName := filepath.Join(config.SolcArtifacts, solc, solc)
	if _, err := os.Stat(fileName); err != nil {
		return "", fmt.Errorf("failed to find compiler %s: %w", version, err)
	}
	return fileName, nil
}
