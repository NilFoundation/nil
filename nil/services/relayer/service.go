package relayer

import (
	"context"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/services/relayer/internal/l1"
	"golang.org/x/sync/errgroup"
)

type RelayerConfig struct {
	EventListenerConfig *l1.EventListenerConfig
}

func DefaultRelayerConfig() *RelayerConfig {
	return &RelayerConfig{
		EventListenerConfig: l1.DefaultEventListenerConfig(),
	}
}

type RelayerService struct {
	L1EventListener *l1.EventListener
}

func New(
	ctx context.Context,
	database db.DB,
	clock common.Timer,
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

	l1Contract, err := l1.NewL1ContractWrapper(l1Client, config.EventListenerConfig.BridgeMessengerContractAddress)
	if err != nil {
		return nil, err
	}

	l1EventListener, err := l1.NewEventListener(
		config.EventListenerConfig,
		l1Client,
		l1Contract,
		l1Storage,
		logger,
	)
	if err != nil {
		return nil, err
	}

	return &RelayerService{
		L1EventListener: l1EventListener,
		// TODO(oclaw) L1 finality ensurer
		// TODO(oclaw) L2 transaction sender
	}, nil
}

func (rs *RelayerService) Run(ctx context.Context) error {
	eg, gCtx := errgroup.WithContext(ctx)

	eventListenerStarted := make(chan struct{})
	eg.Go(func() error {
		return rs.L1EventListener.Run(gCtx, eventListenerStarted)
	})

	return eg.Wait()
}
