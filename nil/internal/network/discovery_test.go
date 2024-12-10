package network

import (
	"context"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/routing"
	"github.com/stretchr/testify/suite"
)

type DiscoverySuite struct {
	networkSuite
}

func (s *DiscoverySuite) SetupSuite() {
	s.port = 1556
}

func (s *DiscoverySuite) SetupTest() {
	s.context, s.ctxCancel = context.WithCancel(context.Background())
}

func (s *DiscoverySuite) TearDownTest() {
	s.ctxCancel()
}

// Test routing via kademlia DHT service.
func (s *DiscoverySuite) TestKadDHT() {
	// setup 3 nodes

	conf := NewDefaultConfig()
	conf.DHTEnabled = true

	m1 := s.newManagerWithBaseConfig(conf)
	s.Require().NotNil(m1.dht)
	defer m1.Close()

	m2 := s.newManagerWithBaseConfig(conf)
	s.Require().NotNil(m2.dht)
	defer m2.Close()

	m3 := s.newManagerWithBaseConfig(conf)
	s.Require().NotNil(m3.dht)
	defer m3.Close()

	// connects node 1 and node 3
	ConnectManagers(s.T(), m3, m1)
	_, err := m1.dht.FindPeer(s.context, m3.host.ID())
	s.Require().NoError(err)

	time.Sleep(startDHTTimeout + 1)

	// node 1 doesn't know about node 2 then should return error
	_, err = m1.dht.FindPeer(s.context, m2.host.ID())
	s.Require().ErrorIs(err, routing.ErrNotFound)

	// connects node 3 and node 2
	ConnectManagers(s.T(), m3, m2)

	time.Sleep(startDHTTimeout + 1)

	// node 1 should know node 2 because both are connected to 3
	_, err = m1.dht.FindPeer(s.context, m2.host.ID())
	s.Require().NoError(err)
}

func (s *DiscoverySuite) TestBeginDiscovery_ThreeNodes() {
	conf := NewDefaultConfig()
	conf.DHTEnabled = true

	nodeA := s.newManagerWithBaseConfig(conf)
	s.Require().NotNil(nodeA.dht)
	defer nodeA.Close()

	nodeB := s.newManagerWithBaseConfig(conf)
	s.Require().NotNil(nodeB.dht)
	defer nodeB.Close()

	nodeC := s.newManagerWithBaseConfig(conf)
	s.Require().NotNil(nodeC.dht)
	defer nodeC.Close()

	// connect A and B
	ConnectManagers(s.T(), nodeA, nodeB)

	// connect A and C
	ConnectManagers(s.T(), nodeA, nodeC)

	time.Sleep(time.Second)

	// assert B and C can discover each other
	addrs := nodeB.host.Peerstore().Addrs(nodeC.host.ID())
	s.Require().NotEmpty(addrs)
}

func TestDiscovery(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(DiscoverySuite))
}
