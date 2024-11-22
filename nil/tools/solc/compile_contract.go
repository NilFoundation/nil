package solc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/abi"
	"github.com/ethereum/go-ethereum/common/compiler"
)

func ParseCombinedJSON(json []byte) (map[string]*compiler.Contract, error) {
	// Provide empty strings for the additional required arguments
	contracts, err := compiler.ParseCombinedJSON(
		json,
		"", /* source */
		"", /* langVersion */
		"", /* compilerVersion */
		"" /* compilerOpts */)
	if err != nil {
		return nil, fmt.Errorf("failed to parse solc output: %w", err)
	}

	res := make(map[string]*compiler.Contract)
	for name, c := range contracts {
		// extract contract name
		contractName := name[strings.LastIndex(name, ":")+1:]
		res[contractName] = c
	}

	return res, nil
}

type compileOptions struct {
	allowedPaths  []string
	basePath      string
	remapping     string
	optimizeParam int
}

func (opts *compileOptions) toArgs(sourceFilePath string) []string {
	args := []string{
		"--combined-json", "abi,bin",
	}

	if len(opts.basePath) > 0 {
		args = append(args, "--base-path", opts.basePath)
	}
	if len(opts.remapping) > 0 {
		args = append(args, opts.remapping)
	}
	if len(opts.allowedPaths) > 0 {
		args = append(args, "--allow-paths")
		args = append(args, strings.Join(opts.allowedPaths, ","))
	}
	if opts.optimizeParam > 0 {
		args = append(args,
			"--optimize",
			"--optimize-runs",
			strconv.Itoa(opts.optimizeParam))
	}

	args = append(args, sourceFilePath)
	return args
}

type CompileOption func(*compileOptions)

func CompileOptionAllowedPaths(paths ...string) CompileOption {
	return func(o *compileOptions) {
		for _, path := range paths {
			o.allowedPaths = append(o.allowedPaths, common.GetAbsolutePath(path))
		}
	}
}

func CompileOptionBasePath(basePath string) CompileOption {
	return func(o *compileOptions) {
		o.basePath = common.GetAbsolutePath(basePath)
	}
}

// allows to use @mylib in .sol files with proper path resolution
func CompileOptionRemapping(from, to string) CompileOption {
	return func(o *compileOptions) {
		o.remapping = fmt.Sprintf("%s=%s", from, common.GetAbsolutePath(to))
		o.basePath = "" // do not needed if we are doing remapping
	}
}

// useful to reduce the size of the compiled contract (limitations are pretty tight)
func CompileOptionOptimizeRuns(val int) CompileOption {
	return func(o *compileOptions) {
		o.optimizeParam = val
	}
}

func CompileSource(sourcePath string, options ...CompileOption) (map[string]*compiler.Contract, error) {
	solc, err := exec.LookPath("solc")
	if err != nil {
		return nil, fmt.Errorf("solc compiler not found: %w", err)
	}

	opts := compileOptions{
		allowedPaths: []string{common.GetAbsolutePath("../../")},
		basePath:     "/",
	}

	for _, o := range options {
		o(&opts)
	}

	args := opts.toArgs(sourcePath)

	cmd := exec.Command(solc, args...)

	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to execute `%s`: %w.\n%s", cmd, err, stderrBuf.String())
	}

	return ParseCombinedJSON(output)
}

func ExtractABI(c *compiler.Contract) abi.ABI {
	data, err := json.Marshal(c.Info.AbiDefinition)
	if err != nil {
		panic(fmt.Errorf("failed to extract abi: %w", err))
	}

	abi, err := abi.JSON(bytes.NewReader(data))
	if err != nil {
		panic(fmt.Errorf("failed to extract abi: %w", err))
	}
	return abi
}
