package relayer

import (
	"context"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/services/relayer/internal/l1"
	"github.com/NilFoundation/nil/nil/services/relayer/internal/l2"
	"github.com/jonboulle/clockwork"
	"golang.org/x/sync/errgroup"
)

type RelayerConfig struct {
	EventListenerConfig           *l1.EventListenerConfig
	FinalityEnsurerConfig         *l1.FinalityEnsurerConfig
	L2BridgeMessengerContractAddr string
}

func DefaultRelayerConfig() *RelayerConfig {
	return &RelayerConfig{
		EventListenerConfig:   l1.DefaultEventListenerConfig(),
		FinalityEnsurerConfig: l1.DefaultFinalityEnsurerConfig(),
	}
}

type RelayerService struct {
	L1EventListener   *l1.EventListener
	L1FinalityEnsurer *l1.FinalityEnsurer
}

func New(
	ctx context.Context,
	database db.DB,
	clock clockwork.Clock,
	config *RelayerConfig,
	l1Client l1.EthClient,
) (*RelayerService, error) {
	logger := logging.NewLogger("relayer")

	l1Storage, err := l1.NewEventStorage(
		ctx,
		database,
		clock,
		nil, // TODO(oclaw) metrics
		logger,
	)
	if err != nil {
		return nil, err
	}

	l1Contract, err := l1.NewL1ContractWrapper(
		l1Client,
		config.EventListenerConfig.BridgeMessengerContractAddress,
		config.L2BridgeMessengerContractAddr,
	)
	if err != nil {
		return nil, err
	}

	l1EventListener, err := l1.NewEventListener(
		config.EventListenerConfig,
		clock,
		l1Client,
		l1Contract,
		l1Storage,
		logger,
	)
	if err != nil {
		return nil, err
	}

	l2Storage := l2.NewEventStorage(
		ctx,
		database,
		clock,
		nil, // TODO(oclaw) metrics
		logger,
	)

	l1FinalityEnsurer, err := l1.NewFinalityEnsurer(
		config.FinalityEnsurerConfig,
		l1Client,
		clock,
		logger,
		l1Storage,
		l2Storage,
		l1EventListener,
	)
	if err != nil {
		return nil, err
	}

	return &RelayerService{
		L1EventListener:   l1EventListener,
		L1FinalityEnsurer: l1FinalityEnsurer,
		// TODO(oclaw) L2 transaction sender
	}, nil
}

func (rs *RelayerService) Run(ctx context.Context) error {
	eg, gCtx := errgroup.WithContext(ctx)

	eventListenerStarted := make(chan struct{})
	eg.Go(func() error {
		return rs.L1EventListener.Run(gCtx, eventListenerStarted)
	})

	finalityEnsurerStarted := make(chan struct{})
	eg.Go(func() error {
		return rs.L1FinalityEnsurer.Run(gCtx, finalityEnsurerStarted)
	})

	return eg.Wait()
}
