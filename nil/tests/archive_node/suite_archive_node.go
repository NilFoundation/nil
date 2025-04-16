//go:build test

package tests

import (
	"context"
	"sync"

	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/network"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/nilservice"
	"github.com/NilFoundation/nil/nil/tests"
)

var (
	oldProtocolVersion = "old"
	newProtocolVersion = "new"
)

func protocolVersionOption(protocolVersion string) network.Option {
	return func(c *network.Config) error {
		c.ProtocolVersion = protocolVersion
		return nil
	}
}

type SuiteArchiveNode struct {
	tests.ShardedSuite

	cancel func()
	wg     sync.WaitGroup

	nShards            uint32
	withBootstrapPeers bool
	port               int
}

func (s *SuiteArchiveNode) SetupSuite() {
	s.nShards = 3
}

func (s *SuiteArchiveNode) newDb() db.DB {
	database, err := db.NewBadgerDbInMemory()
	s.Require().NoError(err)
	return database
}

func (s *SuiteArchiveNode) runArchiveNode(database db.DB) (*nilservice.Config, network.AddrInfo, chan error) {
	s.DbInit = func() db.DB { return database }

	ctx, cancel := context.WithCancel(s.Context)
	s.cancel = cancel
	return s.RunArchiveNode(&tests.ArchiveNodeConfig{
		Ctx:                ctx,
		Wg:                 &s.wg,
		AllowDbDrop:        true,
		Port:               s.port + int(s.nShards),
		WithBootstrapPeers: s.withBootstrapPeers,
		NetworkOptions:     []network.Option{protocolVersionOption(oldProtocolVersion)},
	})
}

func (s *SuiteArchiveNode) startArchiveNode(database db.DB) {
	cfg, addr, rc := s.runArchiveNode(database)
	s.DefaultClient, _ = s.EnsureArchiveNodeStarted(cfg, addr, rc)
}

func (s *SuiteArchiveNode) stopArchiveNode() {
	s.cancel()
	s.wg.Wait()
}

func (s *SuiteArchiveNode) startCluster(protocolVersion string) {
	s.DbInit = nil
	s.Start(
		&nilservice.Config{
			NShards:              s.nShards,
			CollatorTickPeriodMs: 200,
		},
		s.port,
		protocolVersionOption(protocolVersion))
}

func (s *SuiteArchiveNode) checkBlocksGeneration() {
	for shardId := range s.GetNShards() {
		b, err := s.DefaultClient.GetBlock(s.Context, types.ShardId(shardId), 0, true)
		s.Require().NoError(err)
		s.NotNil(b)
	}

	for shardId := range s.GetNShards() {
		tests.WaitBlock(s.T(), s.DefaultClient, types.ShardId(shardId), 1)
	}
}
