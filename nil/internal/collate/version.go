package collate

import (
	"context"
	"errors"
	"fmt"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/network"
	"github.com/NilFoundation/nil/nil/internal/types"
)

const topicVersion = "/nil/version"

type NodeVersion struct {
	ProtocolVersion  string
	GenesisBlockHash common.Hash
}

func SetVersionHandler(ctx context.Context, nm network.Manager, fabric db.DB) error {
	tx, err := fabric.CreateRoTx(ctx)
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

	resp, err := version.MarshalSSZ()
	if err != nil {
		return fmt.Errorf("failed to marshal genesis block hash: %w", err)
	}

	nm.SetRequestHandler(ctx, topicVersion, func(ctx context.Context, _ []byte) ([]byte, error) {
		return resp, nil
	})

	return nil
}

func GetLocalVersion(ctx context.Context, nm network.Manager, fabric db.DB) (NodeVersion, error) {
	protocolVersion := nm.ProtocolVersion()

	tx, err := fabric.CreateRoTx(ctx)
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

func FetchRemoteVersion(ctx context.Context, nm network.Manager, peers network.AddrInfoSlice) (NodeVersion, error) {
	var err error
	for _, peer := range peers {
		var peerId network.PeerID
		peerId, err = nm.Connect(ctx, network.AddrInfo(peer))
		if err != nil {
			continue
		}

		var protocolVersion string
		protocolVersion, err = nm.GetPeerProtocolVersion(peerId)
		if err != nil {
			continue
		}

		var res common.Hash
		res, err = fetchGenesisBlockHash(ctx, nm, peerId)
		if err == nil {
			return NodeVersion{protocolVersion, res}, nil
		}
	}
	return NodeVersion{}, fmt.Errorf("failed to fetch version from all peers; last error: %w", err)
}

func fetchGenesisBlockHash(ctx context.Context, nm network.Manager, peerId network.PeerID) (common.Hash, error) {
	resp, err := nm.SendRequestAndGetResponse(ctx, peerId, topicVersion, nil)
	if err != nil {
		return common.EmptyHash, fmt.Errorf("failed to fetch genesis block hash: %w", err)
	}

	var res common.Hash
	if err := res.UnmarshalSSZ(resp); err != nil {
		return common.EmptyHash, fmt.Errorf("failed to unmarshal genesis block hash: %w", err)
	}

	return res, nil
}
