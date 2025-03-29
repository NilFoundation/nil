package network

import (
	"testing"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/suite"
)

type RelayTestSuite struct {
	networkSuite
}

func (s *RelayTestSuite) SetupSuite() {
	s.port = 1678
}

func (s *RelayTestSuite) TestRelay() {
	// Forcing public reachability, otherwise the relay service will not start
	relay := s.newManagerWithBaseConfig(&Config{
		ServeRelay:   true,
		Reachability: network.ReachabilityPublic,
	})
	defer relay.Close()

	// Forcing private reachability, otherwise the private node won't use the relay
	private := s.newManagerWithBaseConfig(&Config{
		Relays:       []AddrInfo{CalcAddress(relay)},
		Reachability: network.ReachabilityPrivate,
	})
	defer private.Close()

	// Connect the private node to the relay (avoiding discovery)
	ConnectManagers(s.T(), private, relay)

	// The client node must be able to connect to the private node via the relay
	client := s.newManager()
	defer client.Close()

	relayedAddr, err := peer.AddrInfoFromString(hostAddress(relay) + "/p2p-circuit/p2p/" + private.host.ID().String())
	s.Require().NoError(err)

	id, err := client.Connect(s.context, AddrInfo(*relayedAddr))
	s.Require().NoError(err)
	s.Require().Equal(private.host.ID(), id)
}

func TestRelay(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(RelayTestSuite))
}
