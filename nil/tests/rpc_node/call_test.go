package tests

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/NilFoundation/nil/nil/internal/contracts"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/network"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/nilservice"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
	"github.com/NilFoundation/nil/nil/tests"
	"github.com/ethereum/go-ethereum/crypto"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/stretchr/testify/suite"
)

type SuiteRpcNodeCall struct {
	tests.ShardedSuite
}

type NetworkManagerFactory = func(ctx context.Context, cfg *nilservice.Config, db db.DB) (network.Manager, error)

type validatorNetworkManager struct {
	network.Manager

	getPeersForProtocol func(nm network.Manager, protocol network.ProtocolID) []network.PeerID
}

var _ network.Manager = (*validatorNetworkManager)(nil)

func (m *validatorNetworkManager) GetPeersForProtocol(protocol network.ProtocolID) []network.PeerID {
	return m.getPeersForProtocol(m.Manager, protocol)
}

func makeValidatorNetworkManagerFactory(archiveNodePeerId *atomic.Value) NetworkManagerFactory {
	getPeersForProtocol := func(nm network.Manager, protocol network.ProtocolID) []network.PeerID {
		if protocol == "/shard/2/rawapi_ro/Call" {
			peerId, ok := archiveNodePeerId.Load().(network.PeerID)
			check.PanicIfNot(ok && peerId != "")
			return []network.PeerID{peerId}
		}
		return nm.GetPeersForProtocol(protocol)
	}

	return func(ctx context.Context, cfg *nilservice.Config, db db.DB) (network.Manager, error) {
		manager, err := nilservice.CreateNetworkManager(ctx, cfg, db)
		if err != nil {
			return nil, err
		}
		return &validatorNetworkManager{
			Manager:             manager,
			getPeersForProtocol: getPeersForProtocol,
		}, nil
	}
}

func (s *SuiteRpcNodeCall) SetupTest() {
	port := 12001
	nShards := uint32(3)

	var archiveNodePeerId atomic.Value
	s.Start(&nilservice.Config{
		NShards:               nShards,
		RunMode:               nilservice.NormalRunMode,
		NetworkManagerFactory: makeValidatorNetworkManagerFactory(&archiveNodePeerId),
	}, port)

	// Make the archive node lag behind by blocking messages from validators
	withPubSubBlacklist := func(cfg *network.Config) error {
		blacklist := pubsub.NewMapBlacklist()
		for _, i := range s.Instances {
			blacklist.Add(i.P2pAddress.ID)
		}
		cfg.PubSubOptions = append(cfg.PubSubOptions, pubsub.WithBlacklist(blacklist))
		return nil
	}
	_, archiveNodeAddr := s.StartArchiveNode(&tests.ArchiveNodeConfig{
		Port:               port + int(nShards),
		WithBootstrapPeers: false,
		SyncTimeoutFactor:  100500,
		NetworkOptions:     []network.Option{withPubSubBlacklist},
	})
	archiveNodePeerId.Store(archiveNodeAddr.ID)

	s.DefaultClient, _ = s.StartRPCNode(&tests.RpcNodeConfig{WithDhtBootstrapByValidators: true})
}

func (s *SuiteRpcNodeCall) TearDownTest() {
	s.Cancel()
}

func (s *SuiteRpcNodeCall) TestCall() {
	var smartAccountAddress types.Address
	s.Run("Deploy smart account", func() {
		pk, err := crypto.GenerateKey()
		s.Require().NoError(err)
		pubKey := crypto.CompressPubkey(&pk.PublicKey)
		smartAccountCode := contracts.PrepareDefaultSmartAccountForOwnerCode(pubKey)

		var receipt *jsonrpc.RPCReceipt
		smartAccountAddress, receipt = s.DeployContractViaMainSmartAccount(
			types.BaseShardId,
			types.BuildDeployPayload(smartAccountCode, common.EmptyHash),
			types.Value{})
		receipt = s.WaitForReceipt(receipt.TxnHash)
		s.Require().NotNil(receipt)
		s.Require().True(receipt.Success)
	})

	s.Run("EstimateFee", func() {
		code, err := contracts.GetCode(contracts.NameDeployee)
		s.Require().NoError(err)

		abiSmartAccount, err := contracts.GetAbi("SmartAccount")
		s.Require().NoError(err)
		calldata := s.AbiPack(
			abiSmartAccount,
			"asyncDeploy",
			types.NewValueFromUint64(2),
			types.Value0,
			[]byte(code),
			types.Value0)

		callArgs := &jsonrpc.CallArgs{
			Flags: types.NewTransactionFlags(),
			To:    smartAccountAddress,
			Value: types.Value0,
			Data:  (*hexutil.Bytes)(&calldata),
		}

		_, err = s.DefaultClient.EstimateFee(s.Context, callArgs, "latest")
		s.NoError(err, db.ErrKeyNotFound.Error())
	})
}

func TestSuiteRpcNodeCall(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(SuiteRpcNodeCall))
}
