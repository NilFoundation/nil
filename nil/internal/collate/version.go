package collate

import (
	"context"
	"errors"
	"fmt"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/network"
	"github.com/NilFoundation/nil/nil/internal/types"
)

const topicVersion = "/nil/version"

type NodeVersion struct {
	ProtocolVersion  string
	GenesisBlockHash common.Hash
}

type VersionChecker struct {
	nm     network.Manager
	fabric db.DB
	logger logging.Logger
}

func NewVersionChecker(nm network.Manager, fabric db.DB, logger logging.Logger) *VersionChecker {
	return &VersionChecker{
		nm:     nm,
		fabric: fabric,
		logger: logger,
	}
}

func (v *VersionChecker) SetVersionHandler(ctx context.Context) error {
	tx, err := v.fabric.CreateRoTx(ctx)
	if err != nil {
		return fmt.Errorf("failed to create transaction: %w", err)
	}
	defer tx.Rollback()

	// The genesis block must have been initialized before this method is called.
	version, err := db.ReadBlockHashByNumber(tx, types.MainShardId, 0)
	if err != nil {
		return fmt.Errorf("failed to read genesis block hash: %w", err)
	}
	check.PanicIfNot(!version.Empty())

	resp, err := version.MarshalNil()
	if err != nil {
		return fmt.Errorf("failed to marshal genesis block hash: %w", err)
	}

	v.nm.SetRequestHandler(ctx, topicVersion, func(ctx context.Context, _ []byte) ([]byte, error) {
		return resp, nil
	})

	return nil
}

func (v *VersionChecker) GetLocalVersion(ctx context.Context) (NodeVersion, error) {
	protocolVersion := v.nm.ProtocolVersion()

	tx, err := v.fabric.CreateRoTx(ctx)
	if err != nil {
		return NodeVersion{}, err
	}
	defer tx.Rollback()

	res, err := db.ReadBlockHashByNumber(tx, types.MainShardId, 0)
	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			return NodeVersion{protocolVersion, common.EmptyHash}, nil
		}
		return NodeVersion{}, err
	}
	return NodeVersion{protocolVersion, res}, err
}

func (v *VersionChecker) FetchRemoteVersion(ctx context.Context, peers network.AddrInfoSlice) (NodeVersion, error) {
	var err error
	for _, peer := range peers {
		var peerId network.PeerID
		peerId, err = v.nm.Connect(ctx, network.AddrInfo(peer))
		if err != nil {
			v.logger.Debug().Err(err).Msg("failed to fetch remote version from peer")
			continue
		}

		var protocolVersion string
		protocolVersion, err = v.nm.GetPeerProtocolVersion(peerId)
		if err != nil {
			v.logger.Debug().Err(err).
				Stringer(logging.FieldPeerId, peerId).
				Msg("failed to fetch protocol version from peer")
			continue
		}

		var res common.Hash
		res, err = v.fetchGenesisBlockHash(ctx, peerId)
		if err != nil {
			v.logger.Debug().Err(err).
				Stringer(logging.FieldPeerId, peerId).
				Msg("failed to fetch genesis block hash from peer")
			continue
		}

		return NodeVersion{protocolVersion, res}, nil
	}
	return NodeVersion{}, fmt.Errorf("failed to fetch version from all peers; last error: %w", err)
}

func (v *VersionChecker) fetchGenesisBlockHash(ctx context.Context, peerId network.PeerID) (common.Hash, error) {
	resp, err := v.nm.SendRequestAndGetResponse(ctx, peerId, topicVersion, nil)
	if err != nil {
		return common.EmptyHash, fmt.Errorf("failed to fetch genesis block hash: %w", err)
	}

	var res common.Hash
	if err := res.UnmarshalNil(resp); err != nil {
		return common.EmptyHash, fmt.Errorf("failed to unmarshal genesis block hash: %w", err)
	}

	return res, nil
}
