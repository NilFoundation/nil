//go:build test

package tests

import (
	"os"
	"os/exec"
	"strings"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/stretchr/testify/suite"
)

type CliRunner struct {
	suite.Suite
	TmpDir string
}

func (s *CliRunner) RunCli(args ...string) string {
	s.T().Helper()

	data, err := s.RunCliNoCheck(args...)
	s.Require().NoErrorf(err, data)
	return data
}

func (s *CliRunner) RunCliNoCheck(args ...string) (string, error) {
	s.T().Helper()

	if s.TmpDir == "" {
		s.FailNow("TmpDir is not set", "You need to set TmpDir in SetupSuite before use RunCli")
	}

	binPath := s.TmpDir + "/nil.bin"
	if _, err := os.Stat(binPath); err != nil {
		mainPath := common.GetAbsolutePath("../../cmd/nil/main.go")
		cmd := exec.Command("go", "build", "-o", binPath, mainPath)
		s.NoError(cmd.Run())
	}

	cmd := exec.Command(binPath, args...)
	cmd.Env = append(os.Environ(), "INVOCATION_ID=")
	data, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(data)), err
}

func (s *CliRunner) CheckResult(res string, expectedLines ...string) {
	s.T().Helper()

	lines := strings.Split(strings.Trim(res, "\n"), "\n")
	s.Require().GreaterOrEqual(len(lines), len(expectedLines))

	for i, line := range expectedLines {
		s.Require().Equal(line, lines[i])
	}
}
