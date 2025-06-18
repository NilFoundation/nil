package debug

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/relayer/internal/l1"
	"github.com/NilFoundation/nil/nil/services/relayer/internal/l2"
	"github.com/NilFoundation/nil/nil/services/rpc"
	"github.com/NilFoundation/nil/nil/services/rpc/httpcfg"
	"github.com/NilFoundation/nil/nil/services/rpc/transport"
	"github.com/jonboulle/clockwork"
	"golang.org/x/sync/errgroup"
)

const RPCNamespace = "relayerDebug"

type RelayerStats struct {
	// address of the smart account used by relayer to operate on L2
	SmartAccountAddr types.Address ` json:"l2SmartAccountAddr"`

	TimeStamp             time.Time `json:"timestamp"`             // Timestamp of the stats
	PendingL1EventCount   int       `json:"pendingL1EventCount"`   // Fetched from L1 and not finalized events
	PendingL2EventCount   int       `json:"pendingL2EventCount"`   // Finalized events waiting to be sent to L2
	LastProcessedBlock    *uint64   `json:"lastProcessedBlock"`    // Last processed block on L1
	CurrentFinalizedBlock *uint64   `json:"currentFinalizedBlock"` // Current finalized block on L1
}

type Config struct {
	Endpoint string
}

func DefaultConfig() *Config {
	return &Config{
		Endpoint: "",
	}
}

type L1FinalizedBlockGetter interface {
	GetLatestFinalizedBlock() (l1.ProcessedBlock, bool)
}

type RPCListener struct {
	config   *Config
	logger   logging.Logger
	statsAPI *RelayerStatsAPI
}

func NewRPCListener(
	config *Config,
	l1Storage *l1.EventStorage,
	l2Storage *l2.EventStorage,
	finalizedBlockGetter L1FinalizedBlockGetter,
	l2SmartAccountAddr types.Address,
	clock clockwork.Clock,
	logger logging.Logger,
) *RPCListener {
	p := RPCListener{
		config: config,
	}
	p.logger = logger.With().Str(logging.FieldComponent, p.Name()).Logger()
	p.statsAPI = NewRelayerStatsAPI(
		l1Storage,
		l2Storage,
		finalizedBlockGetter,
		l2SmartAccountAddr,
		clock,
		p.logger,
	)
	return &p
}

func (p *RPCListener) Run(ctx context.Context, started chan struct{}) error {
	if p.config.Endpoint == "" {
		p.logger.Info().Msg("Debug info RPC is disabled")
		return nil
	}

	eg, gCtx := errgroup.WithContext(ctx)

	apiStarted := make(chan struct{})
	eg.Go(func() error {
		return p.statsAPI.runRefreshLoop(gCtx, apiStarted)
	})

	listenerStarted := make(chan struct{})
	eg.Go(func() error {
		httpConfig := &httpcfg.HttpCfg{
			HttpURL:         p.config.Endpoint,
			HttpCompression: true,
			TraceRequests:   true,
			HTTPTimeouts:    httpcfg.DefaultHTTPTimeouts,
		}

		apiList := []transport.API{
			{
				Namespace: RPCNamespace,
				Public:    true,
				Service:   p.statsAPI,
				Version:   "1.0",
			},
		}

		p.logger.Info().Msgf("Open relayer stats API endpoint %v", p.config.Endpoint)
		return rpc.StartRpcServer(gCtx, httpConfig, apiList, p.logger, listenerStarted)
	})

	<-apiStarted
	<-listenerStarted
	close(started)

	return eg.Wait()
}

func (p *RPCListener) Name() string {
	return "debug-rpc"
}

type RelayerStatsAPI struct {
	l1Storage              *l1.EventStorage
	l2Storage              *l2.EventStorage
	l1FinalizedBlockGetter L1FinalizedBlockGetter
	smartAccountAddr       types.Address
	clock                  clockwork.Clock
	logger                 logging.Logger

	stats atomic.Pointer[RelayerStats]
}

func NewRelayerStatsAPI(
	l1Storage *l1.EventStorage,
	l2Storage *l2.EventStorage,
	finalizedBlockGetter L1FinalizedBlockGetter,
	l2SmartAccountAddr types.Address,
	clock clockwork.Clock,
	logger logging.Logger,
) *RelayerStatsAPI {
	return &RelayerStatsAPI{
		l1Storage:              l1Storage,
		l2Storage:              l2Storage,
		l1FinalizedBlockGetter: finalizedBlockGetter,
		clock:                  clock,
		logger:                 logger,
		smartAccountAddr:       l2SmartAccountAddr,
		stats:                  atomic.Pointer[RelayerStats]{},
	}
}

func (p *RelayerStatsAPI) runRefreshLoop(ctx context.Context, started chan struct{}) error {
	ticker := p.clock.NewTicker(15 * time.Second)
	defer ticker.Stop()
	close(started)

	if err := p.refresh(ctx); err != nil {
		return err
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.Chan():
			if err := p.refresh(ctx); err != nil {
				p.logger.Error().Err(err).Msg("failed to refresh relayer stats")
			} else {
				p.logger.Debug().Msg("refreshed relayer stats")
			}
		}
	}
}

func (p *RelayerStatsAPI) refresh(ctx context.Context) error {
	now := p.clock.Now()

	l1PendingEvents := 0
	if err := p.l1Storage.IterateEventsByBatch(ctx, 100, func(events []*l1.Event) error {
		l1PendingEvents += len(events)
		return nil
	}); err != nil {
		return err
	}

	l2PendingEvents := 0
	if err := p.l2Storage.IterateEventsByBatch(ctx, 100, func(events []*l2.Event) error {
		l2PendingEvents += len(events)
		return nil
	}); err != nil {
		return err
	}

	stats := &RelayerStats{
		SmartAccountAddr:    p.smartAccountAddr,
		PendingL1EventCount: l1PendingEvents,
		PendingL2EventCount: l2PendingEvents,
		TimeStamp:           now,
	}

	processedBlk, err := p.l1Storage.GetLastProcessedBlock(ctx)
	if err != nil {
		return err
	}
	if processedBlk != nil {
		stats.LastProcessedBlock = &processedBlk.BlockNumber
	}

	finalizedBlk, ok := p.l1FinalizedBlockGetter.GetLatestFinalizedBlock()
	if ok {
		stats.CurrentFinalizedBlock = &finalizedBlk.BlockNumber
	}

	p.stats.Store(stats)
	return nil
}

func (p *RelayerStatsAPI) GetStats() (*RelayerStats, error) {
	ptr := p.stats.Load()
	if ptr == nil {
		return &RelayerStats{}, nil
	}

	return ptr, nil
}
