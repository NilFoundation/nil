package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/packages"
)

// Package RLP is loaded only once and reused for all tests.
var (
	testFset       = token.NewFileSet()
	testImporter   = packagesImporter{fset: testFset}
	testPackageRLP *types.Package
)

type packagesImporter struct {
	fset *token.FileSet
}

func (p packagesImporter) Import(path string) (*types.Package, error) {
	cfg := &packages.Config{
		Fset: p.fset,
		Mode: packages.NeedTypes | packages.NeedImports | packages.NeedTypesInfo,
	}
	pkgs, err := packages.Load(cfg, path)
	if err != nil || len(pkgs) == 0 || pkgs[0].Types == nil {
		return nil, fmt.Errorf("failed to import package %q", path)
	}
	return pkgs[0].Types, nil
}

func init() {
	cfg := &packages.Config{
		Fset: testFset,
		Mode: packages.NeedTypes | packages.NeedImports | packages.NeedSyntax | packages.NeedTypesInfo,
	}

	pkgs, err := packages.Load(cfg, pathOfPackageRLP)
	if err != nil {
		panic(fmt.Errorf("failed to load package %q: %w", pathOfPackageRLP, err))
	}
	if packages.PrintErrors(pkgs) > 0 {
		panic(fmt.Errorf("package %q contains errors", pathOfPackageRLP))
	}
	if len(pkgs) == 0 {
		panic(fmt.Errorf("package %q not found", pathOfPackageRLP))
	}

	testPackageRLP = pkgs[0].Types
}

var tests = []string{
	"uints",
	"nil",
	"rawvalue",
	"optional",
	"bigint",
	"uint256",
	"array",
	"embed",
}

func TestOutput(t *testing.T) {
	t.Parallel()

	for _, test := range tests {
		t.Run(test, func(t *testing.T) {
			t.Parallel()

			inputFile := filepath.Join("testdata", test+".in.txt")
			outputFile := filepath.Join("testdata", test+".out.txt")
			bctx, typ, err := loadTestSource(inputFile, "Test")
			require.NoError(t, err, "error loading test source")

			output, err := bctx.generate(typ, true, true)
			require.NoError(t, err, "error in generate")

			// Set this environment variable to regenerate the test outputs.
			if os.Getenv("WRITE_TEST_FILES") != "" {
				require.NoError(t, os.WriteFile(outputFile, output, 0o600), "error writing test output")
			}

			// Check if output matches.
			wantOutput, err := os.ReadFile(outputFile)
			require.NoError(t, err, "error loading expected test output")
			require.Equal(t, string(wantOutput), string(output), "output mismatch")
		})
	}
}

func loadTestSource(file string, typeName string) (*buildContext, *types.Named, error) {
	// Load the test input.
	content, err := os.ReadFile(file)
	if err != nil {
		return nil, nil, err
	}
	f, err := parser.ParseFile(testFset, file, content, 0)
	if err != nil {
		return nil, nil, err
	}
	conf := types.Config{Importer: testImporter}
	pkg, err := conf.Check("test", testFset, []*ast.File{f}, nil)
	if err != nil {
		return nil, nil, err
	}

	// Find the test struct.
	bctx := newBuildContext(testPackageRLP)
	typ, err := lookupStructType(pkg.Scope(), typeName)
	if err != nil {
		return nil, nil, fmt.Errorf("can't find type %s: %w", typeName, err)
	}
	return bctx, typ, nil
}
