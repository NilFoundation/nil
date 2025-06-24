package relayer

import (
	"context"
	"fmt"
        "encoding/json"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/telemetry"
	"github.com/NilFoundation/nil/nil/services/relayer/internal/debug"
	"github.com/NilFoundation/nil/nil/services/relayer/internal/debug/metrics"
	"github.com/NilFoundation/nil/nil/services/relayer/internal/l1"
	"github.com/NilFoundation/nil/nil/services/relayer/internal/l2"
	"github.com/NilFoundation/nil/nil/services/relayer/internal/storage"
	syncmetrics "github.com/NilFoundation/nil/nil/services/relayer/internal/metrics"
	"github.com/jonboulle/clockwork"
	"golang.org/x/sync/errgroup"
)

type RelayerConfig struct {
	EventListenerConfig     *l1.EventListenerConfig
	FinalityEnsurerConfig   *l1.FinalityEnsurerConfig
	TransactionSenderConfig *l2.TransactionSenderConfig
	L2ContractConfig        *l2.ContractConfig
	TelemetryConfig         *telemetry.Config
	DebugAPIConfig          *debug.Config
	HeartbeatConfig         *debug.HeartbeatConfig
}

func DefaultRelayerConfig() *RelayerConfig {
	return &RelayerConfig{
		EventListenerConfig:     l1.DefaultEventListenerConfig(),
		FinalityEnsurerConfig:   l1.DefaultFinalityEnsurerConfig(),
		TransactionSenderConfig: l2.DefaultTransactionSenderConfig(),
		L2ContractConfig:        l2.DefaultContractConfig(),
		TelemetryConfig: &telemetry.Config{
			ServiceName:   "relayer",
			ExportMetrics: true,
		},
		DebugAPIConfig:  debug.DefaultConfig(),
		HeartbeatConfig: debug.DefaultHeartbeatConfig(),
	}
}

type RelayerService struct {
	Logger              logging.Logger
	Config              *RelayerConfig
	L1EventListener     *l1.EventListener
	L1FinalityEnsurer   *l1.FinalityEnsurer
	L2TransactionSender *l2.TransactionSender
	DebugListener       *debug.RPCListener
	HeartbeatSender     *debug.HeartbeatSender
}

func New(
	ctx context.Context,
	database db.DB,
	clock clockwork.Clock,
	config *RelayerConfig,
	l1Client l1.EthClient,
) (*RelayerService, error) {
	rs := &RelayerService{
		Logger: logging.NewLogger("relayer"),
		Config: config,
	}

	// Log full config as JSON
	cfgJSON, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		rs.Logger.Warn().Err(err).Msg("Failed to marshal relayer config")
	} else {
		rs.Logger.Info().Msgf("Loaded relayer config:\n%s", cfgJSON)
	}

	if err := telemetry.Init(ctx, config.TelemetryConfig); err != nil {
		return nil, fmt.Errorf("failed to init telemetry: %w", err)
	}

	metricsHandler, err := syncmetrics.NewSyncCommitteeMetrics()
	if err != nil {
		return nil, fmt.Errorf("error initializing metrics: %w", err)
	}
	metricsHandler.Heartbeat(ctx)
	storageMetrics, err := storage.NewTableMetrics()
	if err != nil {
		return nil, err
	}

	l1Storage, err := l1.NewEventStorage(
		ctx,
		database,
		clock,
		storageMetrics,
		rs.Logger,
	)
	if err != nil {
		return nil, err
	}

	l1Contract, err := l1.NewL1ContractWrapper(
		l1Client,
		config.EventListenerConfig.BridgeMessengerContractAddress,
		config.EventListenerConfig.L2BridgeAddresses,
		rs.Logger,
	)
	if err != nil {
		return nil, err
	}

	eventListenerMetrics, err := l1.NewEventListenerMetrics()
	if err != nil {
		return nil, err
	}

	rs.L1EventListener, err = l1.NewEventListener(
		config.EventListenerConfig,
		clock,
		l1Client,
		l1Contract,
		l1Storage,
		eventListenerMetrics,
		rs.Logger,
	)
	if err != nil {
		return nil, err
	}

	l2Storage := l2.NewEventStorage(
		ctx,
		database,
		clock,
		storageMetrics,
		rs.Logger,
	)

	finalityEnsurerMetrics, err := l1.NewFinalityEnsurerMetrics()
	if err != nil {
		return nil, err
	}

	rs.L1FinalityEnsurer, err = l1.NewFinalityEnsurer(
		config.FinalityEnsurerConfig,
		l1Client,
		clock,
		rs.Logger,
		l1Storage,
		l2Storage,
		finalityEnsurerMetrics,
		rs.L1EventListener,
	)
	if err != nil {
		return nil, err
	}

	l2Client, l2SmartAccountAddr, err := l2.InitL2(ctx, rs.Logger, config.L2ContractConfig)
	if err != nil {
		return nil, err
	}
	if !l2SmartAccountAddr.IsEmpty() && len(config.L2ContractConfig.SmartAccountAddress) == 0 {
		rs.Logger.Info().
			Str("smart_account_address", l2SmartAccountAddr.Hex()).
			Msg("using automatically created smart account address for L2 operations")
		config.L2ContractConfig.SmartAccountAddress = l2SmartAccountAddr.Hex()
	}

	l2Contract, err := l2.NewL2ContractWrapper(
		ctx,
		config.L2ContractConfig,
		l2Client,
		rs.Logger,
	)
	if err != nil {
		return nil, err
	}

	transactionSenderMetrics, err := l2.NewTransactionSenderMetrics()
	if err != nil {
		return nil, err
	}

	rs.L2TransactionSender, err = l2.NewTransactionSender(
		config.TransactionSenderConfig,
		l2Storage,
		rs.Logger,
		clock,
		rs.L1FinalityEnsurer,
		transactionSenderMetrics,
		l2Contract,
	)
	if err != nil {
		return nil, err
	}

	rs.DebugListener = debug.NewRPCListener(
		config.DebugAPIConfig,
		l1Storage,
		l2Storage,
		rs.L1FinalityEnsurer,
		l2SmartAccountAddr,
		clock,
		rs.Logger,
	)

	metrics, err := metrics.NewHeartbeatMetricHandler()
	if err != nil {
		return nil, err
	}
	rs.HeartbeatSender = debug.NewHeartbeatSender(
		config.HeartbeatConfig,
		metrics,
		clock,
		rs.Logger,
	)

	return rs, nil
}

func (rs *RelayerService) Run(ctx context.Context) error {
	egg, gCtx := errgroup.WithContext(ctx)

	heartbeatStarted := make(chan struct{})
	egg.Go(func() error {
		rs.Logger.Info().Msg("Starting heartbeat sender")
		rs.Logger.Info().Msg("Submitting heartbeat metrics")
		return rs.HeartbeatSender.Run(gCtx, heartbeatStarted, rs.Logger)
	})

	rs.Logger.Info().Msg("relayer: refreshed actual finalized block number")

	eventListenerStarted := make(chan struct{})
	egg.Go(func() error {
		rs.Logger.Info().Msg("Starting L1 event listener")
		rs.Logger.Info().Msg("Submitting L1 event listener metrics")
		return rs.L1EventListener.Run(gCtx, eventListenerStarted)
	})

	finalityEnsurerStarted := make(chan struct{})
	egg.Go(func() error {
		rs.Logger.Info().Msg("Starting finality ensurer")
		rs.Logger.Info().Msg("Submitting finality ensurer metrics")
		return rs.L1FinalityEnsurer.Run(gCtx, finalityEnsurerStarted)
	})

	transactionSenderStarted := make(chan struct{})
	egg.Go(func() error {
		rs.Logger.Info().Msg("Starting L2 transaction sender")
		rs.Logger.Info().Msg("Submitting L2 transaction sender metrics")
		return rs.L2TransactionSender.Run(ctx, transactionSenderStarted)
	})

	<-heartbeatStarted
	<-eventListenerStarted
	<-finalityEnsurerStarted
	<-transactionSenderStarted

	// start debug api after all other services are up and running
	egg.Go(func() error {
		started := make(chan struct{})
		rs.Logger.Info().Msg("Starting debug listener")
		return rs.DebugListener.Run(gCtx, started)
	})

	return egg.Wait()
}
