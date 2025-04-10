package stresser

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"path/filepath"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/NilFoundation/nil/nil/common/concurrent"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/telemetry"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/nilservice"
	"github.com/NilFoundation/nil/nil/services/stresser/core"
	"github.com/NilFoundation/nil/nil/services/stresser/workload"
	"golang.org/x/sync/errgroup"
	"gopkg.in/yaml.v3"
)

var logger = logging.NewLogger("stresser")

const (
	// TODO: should be configurable
	contractsNum     = 96
	workloadInterval = 200 * time.Millisecond
	pollTxInterval   = 4 * time.Second
	maxPendingTxs    = 10000

	modeSingleInstance = "single-instance"
	modeDevnet         = "devnet"
	modeExternal       = "external"
)

type Config struct {
	Mode       string `yaml:"mode"`
	NildPath   string `yaml:"nildPath"`
	WorkingDir string `yaml:"workingDir"`
	Embedded   bool   `yaml:"embedded"`

	// Single instance mode params
	RpcPort   int `yaml:"rpcPort"`
	NumShards int `yaml:"numShards"`

	// External RPC mode params
	RpcEndpoint string `yaml:"endpoint"`

	// Devnet mode params
	DevnetFile string `yaml:"devnetFile"`

	// Number of Stresser contracts to deploy at the beginning
	ContractsNum int `yaml:"contractsNum"`
	// Maximum transactions that can be pending (we are waiting their receipts) at the same time
	MaxPendingTxs int `yaml:"maxPendingTxs"`

	// File with workload configuration
	WorkloadFile string `yaml:"workloadFile"`
	// Embedded workload configuration. It has higher priority than workloadFile.
	Workload any `yaml:"workload"`
}

type Stresser struct {
	cfg             *Config
	nodes           []*NildInstance
	client          *core.Helper
	failedTxsFile   *os.File
	workload        []*workload.Runner
	suspendWorkload atomic.Bool
}

func NewStresserFromFile(configFile string, taskName string) (*Stresser, error) {
	data, err := os.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("can't read config file: %w", err)
	}
	return NewStresser(string(data), filepath.Dir(configFile), taskName)
}

func NewStresser(configYaml string, configPath string, taskName string) (*Stresser, error) {
	var cfg Config
	if err := yaml.Unmarshal([]byte(configYaml), &cfg); err != nil {
		return nil, fmt.Errorf("can't parse config: %w", err)
	}

	if cfg.NumShards <= 1 {
		return nil, errors.New("numShards must be greater than 1")
	}

	s := &Stresser{
		cfg: &cfg,
	}

	if cfg.WorkingDir != "" {
		if err := os.MkdirAll(cfg.WorkingDir, 0o755); err != nil {
			return nil, fmt.Errorf("failed to make dir %s: %w", cfg.WorkingDir, err)
		}
	} else {
		var err error
		cfg.WorkingDir, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to os.Getwd(): %w", err)
		}
	}

	var rpc_endpoint string
	switch cfg.Mode {
	case modeSingleInstance:
		if !cfg.Embedded {
			node := &NildInstance{}
			if err := node.InitSingle(cfg.NildPath, cfg.WorkingDir, cfg.RpcPort, cfg.NumShards); err != nil {
				return nil, fmt.Errorf("failed to init node: %w", err)
			}
			s.nodes = append(s.nodes, node)
		}
		rpc_endpoint = fmt.Sprintf("http://127.0.0.1:%d", cfg.RpcPort)
	case modeDevnet:
		devnetFile := resolveFile(cfg.DevnetFile, configPath)
		cmd := exec.Command(cfg.NildPath, "gen-configs", devnetFile, "--basedir", cfg.WorkingDir) //nolint:gosec
		if out, err := cmd.CombinedOutput(); err != nil {
			return nil, fmt.Errorf("failed to generate devnet config: %w\n %s", err, out)
		}

		if err := s.loadDevnetConfig(devnetFile); err != nil {
			return nil, fmt.Errorf("failed to load devnet config: %w", err)
		}

		for _, node := range s.nodes {
			nodeDir := filepath.Join(cfg.WorkingDir, node.Name)
			if err := node.Init(cfg.NildPath, nodeDir); err != nil {
				return nil, fmt.Errorf("failed to init node: %w", err)
			}
		}
		rpc_endpoint = fmt.Sprintf("http://127.0.0.1:%d", cfg.RpcPort)
	case modeExternal:
		var err error
		s.client, err = core.NewHelper(context.Background(), cfg.RpcEndpoint)
		if err != nil {
			return nil, fmt.Errorf("failed to create client: %w", err)
		}
		rpc_endpoint = cfg.RpcEndpoint
	default:
		return nil, fmt.Errorf("unknown mode: %s", cfg.Mode)
	}

	var err error
	s.client, err = core.NewHelper(context.Background(), rpc_endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	// Create a file for failed transactions dumps
	s.failedTxsFile, err = os.OpenFile(path.Join(cfg.WorkingDir, "failed_txs.txt"),
		os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return nil, fmt.Errorf("failed to create failed_txs.txt file: %w", err)
	}
	logger.Info().Msgf("Failed transactions will be saved to %s", s.failedTxsFile.Name())

	// Init workload
	switch {
	case s.cfg.Workload != nil:
		if err = s.readWorkloadYamlAny(s.cfg.Workload, taskName); err != nil {
			return nil, fmt.Errorf("failed to read workload yaml: %w", err)
		}
	case s.cfg.WorkloadFile != "":
		data, err := os.ReadFile(resolveFile(s.cfg.WorkloadFile, configPath))
		if err != nil {
			return nil, fmt.Errorf("failed to read workload file: %w", err)
		}
		var workload any
		if err = yaml.Unmarshal(data, &workload); err != nil {
			return nil, fmt.Errorf("can't unmarshal workload file: %w", err)
		}
		if err = s.readWorkloadYamlAny(workload, taskName); err != nil {
			return nil, fmt.Errorf("failed to read workload yaml: %w", err)
		}
	default:
		logger.Warn().Msg("Running without workload")
	}

	// Init telemetry
	telemetryCfg := telemetry.NewDefaultConfig()
	telemetryCfg.ExportMetrics = true
	if err := telemetry.Init(context.Background(), telemetryCfg); err != nil {
		return nil, fmt.Errorf("failed to init telemetry: %w", err)
	}

	return s, nil
}

func (s *Stresser) Run(ctx context.Context) error {
	ctx, cancelFn := context.WithCancel(ctx)
	defer cancelFn()

	logger.Info().Msg("Starting stresser")

	wg := &sync.WaitGroup{}

	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGTERM, syscall.SIGINT, syscall.SIGSEGV)
	defer func() {
		cancel()
	}()

	for _, node := range s.nodes {
		if err := node.Run(ctx, wg); err != nil {
			return fmt.Errorf("failed to run node %s", node.Name)
		}
	}

	if s.cfg.Mode == modeSingleInstance && s.cfg.Embedded {
		badger, err := db.NewBadgerDb(path.Join(s.cfg.WorkingDir, "database"))
		if err != nil {
			return fmt.Errorf("failed to create badger db: %w", err)
		}

		nilConfig := nilservice.NewDefaultConfig()
		nilConfig.NShards = uint32(s.cfg.NumShards)
		nilConfig.Telemetry = &telemetry.Config{
			ServiceName:   "stresser",
			ExportMetrics: true,
		}
		nilConfig.RPCPort = 8529
		nilConfig.EnableDevApi = false

		go func() {
			exitCode := nilservice.Run(ctx, nilConfig, badger, nil)
			if exitCode != 0 {
				logger.Error().Int("exit_code", exitCode).Msg("nilservice exited with error")
			} else {
				logger.Info().Msg("nilservice finished successfully")
			}
			cancelFn()
		}()
	}

	logger.Info().Msgf("%d nodes started", len(s.nodes))

	go func() {
		if err := s.runWorkload(ctx); err != nil {
			logger.Error().Err(err).Msg("workload returns error")
		} else {
			logger.Info().Msg("Workload finished successfully")
		}
		cancelFn()
	}()

	if len(s.nodes) != 0 {
		go func() {
			wg.Wait()
			logger.Info().Msg("All nodes are stopped")
			cancelFn()
		}()
	}

	<-ctx.Done()
	s.killAll()

	return nil
}

func (s *Stresser) runWorkload(ctx context.Context) error {
	defer func() {
		if recResult := recover(); recResult != nil {
			s.killAll()
			panic(recResult)
		}
	}()

	logger.Info().Int("workloads_num", len(s.workload)).Msg("Starting workload")

	logger.Info().Int("shards", s.cfg.NumShards).Msg("Waiting for cluster ready...")
	if err := s.client.WaitClusterReady(s.cfg.NumShards); err != nil {
		return fmt.Errorf("failed to wait for rpc node: %w", err)
	}
	logger.Info().Msg("Cluster is ready, start deploying contracts...")

	contracts, err := s.deployContracts()
	if err != nil {
		return fmt.Errorf("failed to deploy contracts: %w", err)
	}

	logger.Info().Msg("All contracts have been successfully deployed")

	workloadArgs := &workload.WorkloadParams{Contracts: contracts, NumShards: s.cfg.NumShards}

	for _, w := range s.workload {
		if err := w.Workload.Init(ctx, s.client, workloadArgs); err != nil {
			return fmt.Errorf("failed to init workload: %w", err)
		}
		if err := w.StartMainLoop(ctx); err != nil {
			return fmt.Errorf("failed to run workload runner: %w", err)
		}
	}

	go s.updateState(ctx)

	logger.Info().Msg("Run workload main loop")

	concurrent.RunTickerLoop(ctx, workloadInterval, func(ctx context.Context) {
		if s.suspendWorkload.Load() {
			return
		}

		for _, w := range s.workload {
			if err := w.RunWorkload(workload.RunParams{}); err != nil {
				logger.Error().Err(err).Msg("failed run workload")
			}
		}
	})
	return nil
}

func (s *Stresser) updateState(ctx context.Context) {
	ticker := time.NewTicker(pollTxInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.printState(ctx)
		}
	}
}

func (s *Stresser) printState(ctx context.Context) {
	totalTxsNum := 0
	for _, w := range s.workload {
		totalTxsNum += w.Workload.TotalTxsNum()
	}

	txPoolStatuses := ""
	for i := 1; i < s.cfg.NumShards; i++ {
		status, err := s.client.Client.GetTxpoolStatus(ctx, types.ShardId(i))
		if err != nil {
			logger.Error().Err(err).Int(logging.FieldShardId, i).Msg("failed to get txpool status")
		}
		txPoolStatuses += fmt.Sprintf("%d:%d.%d ", i, status.Queued, status.Pending)
	}
	logger.Info().Msgf("tx_sent=%d, txpool: %s", totalTxsNum, txPoolStatuses)
}

type WorkloadType []any

func (s *Stresser) readWorkloadYamlAny(workloadAny any, name string) error {
	switch workloadDecoded := workloadAny.(type) {
	case WorkloadType:
		return s.readWorkload(workloadDecoded)
	case map[string]any:
		if name == "" {
			name = "default"
		}
		workloadAny, ok := workloadDecoded[name]
		if !ok {
			return fmt.Errorf("workload %s not found", name)
		}
		workload, ok := workloadAny.([]any)
		if !ok {
			return fmt.Errorf("workload %s must be a list", name)
		}
		return s.readWorkload(workload)
	default:
		return errors.New("workload must be a list or a map")
	}
}

func (s *Stresser) readWorkload(workloadList WorkloadType) error {
	for _, wAny := range workloadList {
		wMap, ok := wAny.(map[string]any)
		if !ok {
			return errors.New("can't parse node")
		}
		data, err := yaml.Marshal(wAny)
		if err != nil {
			return fmt.Errorf("can't marshal workload: %w", err)
		}
		name, ok := wMap["name"].(string)
		if !ok {
			return errors.New("can't parse name")
		}
		wd, err := workload.GetWorkload(name)
		if err != nil {
			return fmt.Errorf("can't get workload: %w", err)
		}
		if err = yaml.Unmarshal(data, wd); err != nil {
			return fmt.Errorf("can't unmarshal %s: %w", name, err)
		}
		s.workload = append(s.workload, workload.NewRunner(wd))
	}
	return nil
}

func (s *Stresser) loadDevnetConfig(file string) error {
	data, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("can't read devnet file: %w", err)
	}
	cfg := make(map[string]any)

	err = yaml.Unmarshal(data, cfg)
	if err != nil {
		return fmt.Errorf("devnet unmashal error: %w", err)
	}
	m, ok := cfg["nil_config"].([]any)
	if !ok {
		return errors.New("can't parse nil_config")
	}
	s.nodes = make([]*NildInstance, 0, len(m)+1)
	for i := range m {
		node := &NildInstance{Name: fmt.Sprintf("nil-%d", i)}
		s.nodes = append(s.nodes, node)
	}
	node := &NildInstance{Name: "nil-rpc-0", IsRpc: true}
	s.nodes = append(s.nodes, node)

	if _, ok := cfg["nil_rpc_config"]; ok {
		s.cfg.RpcPort, ok = cfg["nil_rpc_port"].(int)
		if !ok {
			return errors.New("can't parse nil_rpc_port")
		}
	}
	return nil
}

func (s *Stresser) killAll() {
	for _, node := range s.nodes {
		if node.cmd.ProcessState != nil && node.cmd.ProcessState.Exited() {
			continue
		}
		if err := node.cmd.Process.Kill(); err != nil {
			logger.Error().Err(err).Msg("failed to kill node")
		}
	}
}

func (s *Stresser) deployContracts() ([]*core.Contract, error) {
	contracts := make([]*core.Contract, 0, contractsNum)
	numShards := (s.cfg.NumShards - 1)

	var g errgroup.Group

	contractsPerShard := contractsNum / numShards
	contractsChan := make(chan []*core.Contract, numShards)
	defer close(contractsChan)

	for shardId := 1; shardId < s.cfg.NumShards; shardId++ {
		g.Go(func() error {
			stresses, err := s.client.DeployStressers(types.ShardId(shardId), contractsPerShard)
			if err != nil {
				logger.Error().Err(err).Msgf("failed to deploy stresses on shard %d", shardId)
				return err
			}
			contractsChan <- stresses
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, fmt.Errorf("failed to deploy contracts: %w", err)
	}
	if len(contractsChan) != numShards {
		return nil, errors.New("not all shards returned deployed contracts")
	}
	shardContracts := make([][]*core.Contract, numShards)
	for i := range numShards {
		c := <-contractsChan
		shardContracts[i] = c
	}

	for i := range contractsPerShard {
		for _, c := range shardContracts {
			contracts = append(contracts, c[i])
		}
	}

	return contracts, nil
}

func resolveFile(name string, basePath string) string {
	var fileResolved string
	if filepath.IsAbs(name) {
		fileResolved = name
	} else {
		fileResolved = path.Join(basePath, name)
	}
	return fileResolved
}
