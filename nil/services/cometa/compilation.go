package cometa

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/fabelx/go-solc-select/pkg/config"
	"github.com/fabelx/go-solc-select/pkg/installer"
	"github.com/fabelx/go-solc-select/pkg/versions"
)

const (
	GeneratedSourceFileName = "generated"
)

type ContractData struct {
	Name            string                  `json:"name,omitempty"`
	Description     string                  `json:"description,omitempty"`
	Abi             []any                   `json:"abi,omitempty"`
	SourceCode      map[string]string       `json:"sourceCode,omitempty"`
	SourceMap       string                  `json:"sourceMap,omitempty"`
	Metadata        string                  `json:"metadata,omitempty"`
	DeployCode      []byte                  `json:"deployCode,omitempty"`
	Code            []byte                  `json:"code,omitempty"`
	SourceFilesList []string                `json:"sourceFilesList,omitempty"`
	CompilerOutput  *CompilerOutputContract `json:"compilerOutput,omitempty"`
}

func Compile(inputJson string) (*ContractData, error) {
	logger.Info().Msg("Start contract compiling...")
	dir, err := os.MkdirTemp("/tmp", "compilation_")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(dir)

	var input JsonInput
	if err = json.Unmarshal([]byte(inputJson), &input); err != nil {
		return nil, fmt.Errorf("failed to unmarshal input json: %w", err)
	}

	solc, err := findCompiler(input.CompilerVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to find compiler: %w", err)
	}

	compilerInput, err := input.ToCompilerJsonInput()
	if err != nil {
		return nil, fmt.Errorf("failed to convert input to compiler input: %w", err)
	}
	compilerInputStr, err := json.MarshalIndent(compilerInput, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal compiler input: %w", err)
	}

	inputFile := dir + "/input.json"
	if err = os.WriteFile(inputFile, compilerInputStr, 0o600); err != nil {
		return nil, fmt.Errorf("failed to write input file: %w", err)
	}

	args := []string{"--standard-json", inputFile, "--pretty-json"}
	cmd := exec.Command(solc, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("Compilation failed:\n%s\n", output)
		return nil, err
	}

	var outputJson CompilerJsonOutput
	if err := json.Unmarshal(output, &outputJson); err != nil {
		fmt.Printf("Failed to unmarshal json: %s\n", err.Error())
		return nil, err
	}
	if len(outputJson.Errors) != 0 {
		fmt.Printf("Compilation failed:\n%s\n", output)
		return nil, fmt.Errorf("compilation failed: %s", outputJson.Errors[0].FormattedMessage)
	}

	contractData, err := LoadContractInfo(compilerInput, &outputJson)
	if err != nil {
		return nil, fmt.Errorf("failed to load contract info: %w", err)
	}
	contractData.Name = input.ContractName

	return contractData, nil
}

func LoadContractInfo(input *CompilerJsonInput, outputJson *CompilerJsonOutput) (*ContractData, error) {
	contractData := &ContractData{}

	contractData.SourceFilesList = make([]string, len(input.Sources)+1)
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
	if len(contractData.SourceFilesList[len(contractData.SourceFilesList)-1]) != 0 {
		return nil, errors.New("last id must be empty")
	}
	contractData.SourceFilesList[len(contractData.SourceFilesList)-1] = GeneratedSourceFileName

	var contractDescr *CompilerOutputContract
	for _, v := range outputJson.Contracts {
		if len(v) > 1 {
			return nil, errors.New("expected exactly one contract in compilation output")
		}
		if len(v) != 0 {
			for _, c := range v {
				contractDescr = &c
				break
			}
			break
		}
	}
	if contractDescr == nil {
		return nil, errors.New("contract not found in compilation output")
	}
	contractData.CompilerOutput = contractDescr

	contractData.SourceMap = contractDescr.Evm.DeployedBytecode.SourceMap
	if len(contractData.SourceMap) == 0 {
		return nil, errors.New("source map not found")
	}

	contractData.Metadata = contractDescr.Metadata
	contractData.Code = hexutil.MustDecode(contractDescr.Evm.DeployedBytecode.Object)
	contractData.DeployCode = hexutil.MustDecode(contractDescr.Evm.Bytecode.Object)
	contractData.Abi = contractDescr.Abi

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
