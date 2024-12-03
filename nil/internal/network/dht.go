package network

import (
	"context"
	"time"

	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/rs/zerolog"
)

type DHT = dht.IpfsDHT

func NewDHT(ctx context.Context, h host.Host, conf *Config, logger zerolog.Logger) (*DHT, error) {
	if !conf.DHTEnabled {
		return nil, nil
	}

	logger.Debug().Msg("Starting DHT")

	if len(conf.DHTBootstrapPeers) == 0 {
		logger.Info().Msg("No bootstrap peers provided, starting DHT in server mode")
		return dht.New(ctx, h, dht.Mode(dht.ModeServer))
	}

	res, err := dht.New(ctx, h, dht.BootstrapPeers(ToLibP2pAddrInfoSlice(conf.DHTBootstrapPeers)...), dht.RoutingTableRefreshPeriod(1*time.Minute))
	if err != nil {
		return nil, err
	}
	if err := res.Bootstrap(ctx); err != nil {
		return nil, err
	}

	logger.Info().Msgf("DHT bootstrapped with %d peers", len(conf.DHTBootstrapPeers))

	return res, nil
}
