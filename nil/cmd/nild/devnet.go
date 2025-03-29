//nolint:tagliatelle
package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/NilFoundation/nil/nil/cmd/nild/nildconfig"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/internal/config"
	nilcrypto "github.com/NilFoundation/nil/nil/internal/crypto"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/keys"
	"github.com/NilFoundation/nil/nil/internal/network"
	"github.com/NilFoundation/nil/nil/internal/telemetry"
	"github.com/NilFoundation/nil/nil/services/nilservice"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

type nodeSpec struct {
	ID                   int    `yaml:"id"`
	Shards               []uint `yaml:"shards"`
	SplitShards          bool   `yaml:"splitShards"`
	BootstrapPeersIdx    []int  `yaml:"bootstrapPeersIdx"`
	DHTBootstrapPeersIdx []int  `yaml:"dhtBootstrapPeersIdx"`
	ArchiveNodeIndices   []int  `yaml:"archiveNodeIndices"`
}

type clusterSpec struct {
	NilServerName          string   `yaml:"nil_server_name"`
	NilCertEmail           string   `yaml:"nil_cert_email"`
	NildConfigDir          string   `yaml:"nild_config_dir"`
	NildCredentialsDir     string   `yaml:"nild_credentials_dir"`
	NildPromBasePort       int      `yaml:"nild_prom_base_port"`
	NildP2PBaseTCPPort     int      `yaml:"nild_p2p_base_tcp_port"`
	PprofBaseTCPPort       int      `yaml:"pprof_base_tcp_port"`
	NilWipeOnUpdate        bool     `yaml:"nil_wipe_on_update"`
	NShards                uint32   `yaml:"nShards"`
	NilRPCHost             string   `yaml:"nil_rpc_host"`
	NilRPCPort             int      `yaml:"nil_rpc_port"`
	EnableRPCOnValidators  bool     `yaml:"nil_rpc_enable_on_validators"`
	ClickhouseHost         string   `yaml:"clickhouse_host"`
	ClickhousePort         int      `yaml:"clickhouse_port"`
	ClickhouseLogin        string   `yaml:"clickhouse_login"`
	ClickhouseDatabase     string   `yaml:"clickhouse_database"`
	CometaRPCHost          string   `yaml:"cometa_rpc_host"`
	CometaPort             int      `yaml:"cometa_port"`
	FaucetRPCHost          string   `yaml:"faucet_rpc_host"`
	FaucetPort             int      `yaml:"faucet_port"`
	NilLoadgenHost         string   `yaml:"nil_loadgen_host"`
	NilLoadgenPort         int      `yaml:"nil_loadgen_port"`
	NilUpdateRetryInterval int      `yaml:"nil_update_retry_interval_sec"`
	InstanceEnv            string   `yaml:"instance_env"`
	SignozJournaldLogs     []string `yaml:"signoz_journald_logs"`

	NilConfig        []nodeSpec `yaml:"nil_config"`
	NilArchiveConfig []nodeSpec `yaml:"nil_archive_config"`
	NilRPCConfig     []nodeSpec `yaml:"nil_rpc_config"`

	NilLoadGeneratorsEnable bool `yaml:"nil_load_generators_enable"`
}

type server struct {
	service         string
	name            string
	identity        string
	p2pPort         int
	promPort        int
	pprofPort       int
	rpcPort         int
	credsDir        string
	workDir         string
	nodeSpec        nodeSpec
	logClientEvents bool
	vkm             *keys.ValidatorKeysManager
}

type cluster struct {
	spec       *clusterSpec
	baseDir    string
	validators []server
	archivers  []server
	rpcNodes   []server
	zeroState  *execution.ZeroStateConfig
}

func DevnetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "gen-configs",
		Short:        "Generate devnet configuration",
		Args:         cobra.ExactArgs(1),
		RunE:         genDevnet,
		SilenceUsage: true,
	}
	cmd.Flags().String("basedir", "/var/lib", "Base directory for devnet")
	cmd.Flags().String("only", "", "Write only specified service (e.g. nil-archive-1)")
	return cmd
}

func validatorKeysFile(credsDir string) string {
	return filepath.Join(credsDir, "validator-keys.yaml")
}

func (spec *clusterSpec) ensureValidatorKeys(srv *server) (*keys.ValidatorKeysManager, error) {
	vkm := keys.NewValidatorKeyManager(validatorKeysFile(srv.credsDir))
	if err := vkm.InitKey(); err != nil {
		return nil, err
	}
	return vkm, nil
}

func (c *cluster) generateZeroState(nShards uint32, servers []server) (*execution.ZeroStateConfig, error) {
	validators := make([]config.ListValidators, nShards-1)
	for _, srv := range servers {
		key, err := srv.vkm.GetPublicKey()
		if err != nil {
			return nil, err
		}

		for _, id := range srv.nodeSpec.Shards {
			if id == 0 {
				continue
			}
			idx := id - 1
			validators[idx].List = append(validators[idx].List, config.ValidatorInfo{
				PublicKey: config.Pubkey(key),
			})
		}
	}

	mainKeyPath := c.spec.NildCredentialsDir + "/keys.yaml"
	mainPublicKey, err := ensurePublicKey(mainKeyPath)
	if err != nil {
		return nil, err
	}

	zeroState, err := execution.CreateDefaultZeroStateConfig(mainPublicKey)
	if err != nil {
		return nil, err
	}
	zeroState.ConfigParams.Validators = config.ParamValidators{Validators: validators}

	return zeroState, nil
}

func ensurePublicKey(keyPath string) ([]byte, error) {
	privateKey, err := execution.LoadMainKeys(keyPath)
	if err == nil {
		publicKey := crypto.CompressPubkey(&privateKey.PublicKey)
		return publicKey, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		// if the file exists but is invalid, return the error
		return nil, err
	}

	privateKey, publicKey, err := nilcrypto.GenerateKeyPair()
	if err != nil {
		return nil, err
	}
	if err := execution.DumpMainKeys(keyPath, privateKey); err != nil {
		return nil, err
	}
	return publicKey, nil
}

func genDevnet(cmd *cobra.Command, args []string) error {
	baseDir, err := cmd.Flags().GetString("basedir")
	if err != nil {
		return fmt.Errorf("failed to get basedir flag: %w", err)
	}
	baseDir, err = filepath.Abs(baseDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for basedir: %w", err)
	}

	specFile := args[0]
	specYaml, err := os.ReadFile(specFile)
	if err != nil {
		return fmt.Errorf("can't read devnet spec %s: %w", specFile, err)
	}

	spec := &clusterSpec{}
	if err := yaml.Unmarshal(specYaml, spec); err != nil {
		return fmt.Errorf("can't parse devnet spec %s: %w", specFile, err)
	}

	validatorRPCBasePort := 0
	if spec.EnableRPCOnValidators {
		validatorRPCBasePort = spec.NilRPCPort + len(spec.NilRPCConfig)
	}
	validators, err := spec.makeServers(spec.NilConfig,
		spec.NildP2PBaseTCPPort, spec.NildPromBasePort, spec.PprofBaseTCPPort, validatorRPCBasePort,
		"nil", baseDir, false)
	if err != nil {
		return fmt.Errorf("failed to setup validator nodes: %w", err)
	}

	c := &cluster{spec: spec, baseDir: baseDir, validators: validators}

	archiveBaseP2P := spec.NildP2PBaseTCPPort + len(validators)
	archiveBaseProm := spec.NildPromBasePort + len(validators)
	archiveBasePprof := spec.PprofBaseTCPPort + len(validators)

	c.archivers, err = spec.makeServers(spec.NilArchiveConfig,
		archiveBaseP2P, archiveBaseProm, archiveBasePprof, 0,
		"nil-archive", baseDir, false)
	if err != nil {
		return fmt.Errorf("failed to setup archive nodes: %w", err)
	}

	rpcBasePprof := spec.PprofBaseTCPPort + len(validators) + len(c.archivers)

	c.rpcNodes, err = spec.makeServers(spec.NilRPCConfig,
		0, 0, rpcBasePprof, spec.NilRPCPort,
		"nil-rpc", baseDir, true)
	if err != nil {
		return fmt.Errorf("failed to setup rpc nodes: %w", err)
	}

	only, err := cmd.Flags().GetString("only")
	if err != nil {
		return fmt.Errorf("failed to get only flag: %w", err)
	}

	if c.zeroState, err = c.generateZeroState(spec.NShards, c.validators); err != nil {
		return err
	}
	if err := c.writeConfigs(c.validators, "validator", only); err != nil {
		return err
	}
	if err := c.writeConfigs(c.archivers, "archiver", only); err != nil {
		return err
	}
	if err := c.writeConfigs(c.rpcNodes, "RPC node", only); err != nil {
		return err
	}

	os.Exit(0)
	return nil
}

func (spec *clusterSpec) EnsureValidatorKeys(srv server) (*keys.ValidatorKeysManager, error) {
	if err := os.MkdirAll(srv.credsDir, directoryPermissions); err != nil {
		return nil, err
	}

	return spec.ensureValidatorKeys(&srv)
}

func (c *cluster) writeConfigs(servers []server, name string, only string) error {
	for i, server := range servers {
		if err := c.writeServerConfig(i, server, only); err != nil {
			return fmt.Errorf("failed to write %s config: %w", name, err)
		}
	}
	return nil
}

func (spec *clusterSpec) makeServers(
	nodeSpecs []nodeSpec,
	baseP2pPort int,
	basePromPort int,
	basePprofPort int,
	baseHTTPPort int,
	service string,
	baseDir string,
	logClientEvents bool,
) ([]server, error) {
	servers := make([]server, len(nodeSpecs))
	for i, nodeSpec := range nodeSpecs {
		servers[i].service = service
		servers[i].name = fmt.Sprintf("%s-%d", service, i)
		servers[i].nodeSpec = nodeSpec
		servers[i].logClientEvents = logClientEvents
		if baseP2pPort != 0 {
			servers[i].p2pPort = baseP2pPort + i
		}
		if basePromPort != 0 {
			servers[i].promPort = basePromPort + i
		}
		if basePprofPort != 0 {
			servers[i].pprofPort = basePprofPort + i
		}
		if baseHTTPPort != 0 {
			servers[i].rpcPort = baseHTTPPort + i
		}
		servers[i].credsDir = fmt.Sprintf("%s/%s-%d", spec.NildCredentialsDir, service, i)
		servers[i].workDir = fmt.Sprintf("%s/%s-%d", baseDir, service, i)

		var err error
		servers[i].identity, err = spec.EnsureIdentity(servers[i])
		if err != nil {
			return nil, err
		}
		servers[i].vkm, err = spec.EnsureValidatorKeys(servers[i])
		if err != nil {
			return nil, err
		}
	}
	return servers, nil
}

func (srv server) NetworkKeysFile() string {
	return srv.credsDir + "/network-keys.yaml"
}

const (
	directoryPermissions = 0o755
	filePermissions      = 0o644
)

func (spec *clusterSpec) EnsureIdentity(srv server) (string, error) {
	if err := os.MkdirAll(srv.credsDir, directoryPermissions); err != nil {
		return "", err
	}
	privKey, err := network.LoadOrGenerateKeys(srv.NetworkKeysFile())
	if err != nil {
		return "", fmt.Errorf("failed to load or generate keys: %w", err)
	}
	_, _, identity, err := network.SerializeKeys(privKey)
	return identity.String(), err
}

func (c *cluster) writeServerConfig(instanceId int, srv server, only string) error {
	if only != "" && srv.name != only {
		return nil
	}

	spec := c.spec
	inst := srv.nodeSpec

	cfg := nildconfig.Config{
		Config: &nilservice.Config{
			NShards:            spec.NShards,
			AllowDbDrop:        spec.NilWipeOnUpdate,
			LogClientRpcEvents: srv.logClientEvents,

			MyShards:       inst.Shards,
			SplitShards:    inst.SplitShards,
			BootstrapPeers: getPeers(c.validators, inst.BootstrapPeersIdx),

			RPCPort:         srv.rpcPort,
			PprofPort:       srv.pprofPort,
			AdminSocketPath: srv.workDir + "/admin_socket",

			ZeroState: c.zeroState,

			Network: &network.Config{
				TcpPort: srv.p2pPort,

				DHTEnabled:        true,
				DHTBootstrapPeers: getPeers(c.validators, inst.DHTBootstrapPeersIdx),
			},
			Telemetry: &telemetry.Config{
				ExportMetrics:  true,
				PrometheusPort: srv.promPort,
			},
		},
		DB: db.NewDefaultBadgerDBOptions(),
	}

	var err error
	cfg.Network.KeysPath, err = filepath.Abs(srv.NetworkKeysFile())
	if err != nil {
		return fmt.Errorf("failed to get absolute path for network keys: %w", err)
	}

	cfg.ValidatorKeysPath, err = filepath.Abs(validatorKeysFile(srv.credsDir))
	if err != nil {
		return fmt.Errorf("failed to get absolute path for validator keys: %w", err)
	}

	if len(inst.ArchiveNodeIndices) > 0 {
		cfg.RpcNode = &nilservice.RpcNodeConfig{
			ArchiveNodeList: getPeers(c.archivers, inst.ArchiveNodeIndices),
		}
	}

	serialized, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.MkdirAll(srv.workDir, directoryPermissions); err != nil {
		return err
	}

	configDir := fmt.Sprintf("%s/%s-%d", spec.NildConfigDir, srv.service, instanceId)
	if err := os.MkdirAll(configDir, directoryPermissions); err != nil {
		return err
	}

	return os.WriteFile(configDir+"/nild.yaml", serialized, filePermissions)
}

func identityToAddress(port int, identity string) string {
	return fmt.Sprintf("/ip4/127.0.0.1/tcp/%d/p2p/%s", port, identity)
}

func getPeer(srv server) network.AddrInfo {
	var peer network.AddrInfo
	address := identityToAddress(srv.p2pPort, srv.identity)
	check.PanicIfErr(peer.Set(address))
	return peer
}

func getPeers(servers []server, indices []int) network.AddrInfoSlice {
	peers := make(network.AddrInfoSlice, len(indices))
	for i, idx := range indices {
		peers[i] = getPeer(servers[idx])
	}
	return peers
}
