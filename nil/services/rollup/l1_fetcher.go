package rollup

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/NilFoundation/nil/nil/common/concurrent"
	"github.com/NilFoundation/nil/nil/common/logging"
	l1types "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rpc"
)

//go:generate go run github.com/matryer/moq -out l1_fetcher_generated_mock.go -rm -stub -with-resets . L1BlockFetcher

const pollInterval = 5 * time.Second

type L1BlockFetcher interface {
	GetLastBlockInfo(ctx context.Context) (*l1types.Header, error)
}

type L1BlockFetcherRpc struct {
	client    *rpc.Client
	header    *l1types.Header
	logger    logging.Logger
	nodeIndex int
	lock      sync.RWMutex
}

var rpcNodeList = []string{
	"https://eth.llamarpc.com",
	"https://eth-mainnet.public.blastapi.io",
	"https://rpc.ankr.com/eth",
	"https://eth-mainnet.public.blastapi.io",
	"https://rpc.flashbots.net",
	"https://cloudflare-eth.com",
}

func NewL1BlockFetcherRpc(ctx context.Context) L1BlockFetcher {
	p := &L1BlockFetcherRpc{
		logger: logging.NewLogger("l1fetcher"),
	}
	go func() {
		p.Run(ctx)
	}()
	return p
}

func (p *L1BlockFetcherRpc) GetLastBlockInfo(ctx context.Context) (*l1types.Header, error) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	if p.header == nil {
		return nil, errors.New("no blocks have been fetched yet")
	}
	return p.header, nil
}

func (p *L1BlockFetcherRpc) Run(ctx context.Context) {
	p.fetch()
	concurrent.RunTickerLoop(ctx, pollInterval, func(ctx context.Context) {
		p.fetch()
	})
}

func (p *L1BlockFetcherRpc) fetch() {
	if p.client == nil {
		if err := p.connect(); err != nil {
			p.logger.Error().Err(err).Msg("failed to connect to L1")
			return
		}
	}

	var header *l1types.Header
	if err := p.client.Call(&header, "eth_getBlockByNumber", "latest", false); err != nil {
		p.logger.Warn().
			Err(err).
			Str("node", rpcNodeList[p.nodeIndex]).
			Msg("failed to get L1 last block")
		if err = p.switchNode(); err != nil {
			p.logger.Error().Err(err).Msg("failed to switch node")
		}
	} else {
		p.lock.Lock()
		defer p.lock.Unlock()
		p.header = header
	}
}

func (p *L1BlockFetcherRpc) connect() error {
	if p.client != nil {
		p.client.Close()
	}
	var err error
	if p.client, err = rpc.Dial(rpcNodeList[p.nodeIndex]); err != nil {
		return fmt.Errorf("failed to connect to L1: %w", err)
	}
	return nil
}

func (p *L1BlockFetcherRpc) switchNode() error {
	initialIndex := p.nodeIndex
	for {
		if p.nodeIndex == len(rpcNodeList)-1 {
			p.nodeIndex = 0
		} else {
			p.nodeIndex++
		}
		p.logger.Info().Msgf("Switch to new provider: %s", rpcNodeList[p.nodeIndex])
		err := p.connect()
		if err == nil {
			break
		}
		if p.nodeIndex == initialIndex {
			return fmt.Errorf("all nodes are down, last error: %w", err)
		}
	}
	return nil
}
