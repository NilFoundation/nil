package stresser

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"

	"github.com/NilFoundation/nil/nil/common/logging"
)

type NildInstance struct {
	Name       string
	IsRpc      bool
	workingDir string
	cmd        *exec.Cmd
	logger     logging.Logger
}

var nildIndex = 0

func (n *NildInstance) Init(nildPath string, workingDir string) error {
	n.logger = logging.NewLogger(n.Name)

	n.workingDir = workingDir
	if err := os.MkdirAll(n.workingDir, 0o755); err != nil {
		return fmt.Errorf("failed to create working directory: %w", err)
	}

	cfgFile := filepath.Join(n.workingDir, "nild.yaml")

	cmd := "run"
	if n.IsRpc {
		cmd = "rpc"
	}

	const profilePortBase = 6000
	profPort := profilePortBase + nildIndex

	nildIndex++
	n.cmd = exec.Command(nildPath, cmd, "--config", cfgFile, "-l", "warn", "--log-filter", "-consensus", "--dev-api",
		"--pprof-port", strconv.Itoa(profPort))
	n.cmd.Dir = n.workingDir

	return nil
}

func (n *NildInstance) InitSingle(nildPath string, rootDir string, rpcPort int, numShards int) error {
	n.Name = "nil-single"
	n.logger = logging.NewLogger("nil")
	if rootDir == "" {
		var err error
		rootDir, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get working directory: %w", err)
		}
	}

	n.workingDir = rootDir
	if err := os.MkdirAll(n.workingDir, 0o755); err != nil {
		return fmt.Errorf("failed to create working directory: %w", err)
	}

	n.cmd = exec.Command(nildPath, "run", "--nshards", strconv.Itoa(numShards), "--http-port", //nolint:gosec
		strconv.Itoa(rpcPort), "-l", "debug", "--log-filter", "-consensus", "--dev-api")
	n.cmd.Dir = n.workingDir

	return nil
}

func (n *NildInstance) Run(ctx context.Context, wg *sync.WaitGroup) error {
	if n.cmd == nil {
		return nil
	}

	// Set process group to allow killing the entire group
	n.cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	wg.Add(1)

	logFile, err := os.Create(filepath.Join(n.workingDir, "output.log"))
	if err != nil {
		return fmt.Errorf("error opening log file: %w", err)
	}
	n.cmd.Stdout = logFile
	n.cmd.Stderr = logFile

	if err = n.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start process: %w", err)
	}

	exitEvent := make(chan struct{})

	go func() {
		err := n.cmd.Wait()
		if err != nil {
			n.logger.Error().Err(err).Int("pid", n.cmd.Process.Pid).Msg("Process exited with error")
		} else {
			n.logger.Info().Msg("Process exited successfully")
		}
		close(exitEvent)
	}()

	go func() {
		select {
		case <-ctx.Done():
			n.logger.Error().Msg("Stopping node")
		case <-exitEvent:
			n.logger.Info().Msg("Process has stopped")
		}
		wg.Done()
	}()

	n.logger.Info().Str("command", n.cmd.String()).Int("pid", n.cmd.Process.Pid).Msg("Started process")

	return nil
}
