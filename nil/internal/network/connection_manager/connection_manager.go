package connection_manager

import (
	"context"

	"github.com/NilFoundation/nil/nil/common/logging"
	libp2pconnmgr "github.com/libp2p/go-libp2p/core/connmgr"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/p2p/net/connmgr"
)

type withCustomNotifeeDecorator struct {
	*connmgr.BasicConnMgr

	notifee network.Notifiee
}

func (cm *withCustomNotifeeDecorator) Notifee() network.Notifiee {
	return cm.notifee
}

var (
	_ libp2pconnmgr.ConnManager = (*withCustomNotifeeDecorator)(nil)
	_ libp2pconnmgr.Decayer     = (*withCustomNotifeeDecorator)(nil)
)

func NewConnectionManagerWithPeerReputationTracking(
	ctx context.Context,
	conf *Config,
	logger logging.Logger,
	low, hi int,
	opts ...connmgr.Option,
) (libp2pconnmgr.ConnManager, error) {
	baseConnectionManager, err := connmgr.NewConnManager(low, hi, opts...)
	if err != nil {
		return nil, err
	}
	notifee := newNotifiee(baseConnectionManager.Notifee(), conf, logger)
	notifee.start(ctx)
	return &withCustomNotifeeDecorator{
		BasicConnMgr: baseConnectionManager,
		notifee:      notifee,
	}, nil
}
