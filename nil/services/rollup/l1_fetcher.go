package rollup

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/NilFoundation/nil/nil/common/concurrent"
	"github.com/NilFoundation/nil/nil/common/logging"
	l1types "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/rs/zerolog"
)

//go:generate go run github.com/matryer/moq -out l1_fetcher_generated_mock.go -rm -stub -with-resets . L1BlockFetcher

const pollInterval = 5 * time.Second

type L1BlockFetcher interface {
	GetLastBlockInfo(ctx context.Context) (*l1types.Header, error)
}

type L1BlockFetcherRpc struct {
	ctx       context.Context
	client    *rpc.Client
	header    *l1types.Header
	logger    zerolog.Logger
	nodeIndex int
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
		ctx:    ctx,
		logger: logging.NewLogger("l1fetcher"),
	}
	go func() {
		p.Run()
	}()
	return p
}

func (p *L1BlockFetcherRpc) GetLastBlockInfo(ctx context.Context) (*l1types.Header, error) {
	if p.header == nil {
		return nil, errors.New("no blocks have been fetched yet")
	}
	return p.header, nil
}

func (p *L1BlockFetcherRpc) Run() {
	p.fetch()
	concurrent.RunTickerLoop(p.ctx, pollInterval, func(ctx context.Context) {
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
	err := p.client.Call(&p.header, "eth_getBlockByNumber", "latest", false)
	if err != nil {
		p.logger.Warn().
			Err(err).
			Str("node", rpcNodeList[p.nodeIndex]).
			Msg("failed to get L1 last block")
		if err = p.switchNode(); err != nil {
			p.logger.Error().Err(err).Msg("failed to switch node")
		}
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
