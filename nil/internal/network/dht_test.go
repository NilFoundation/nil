package network

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type DHTSuite struct {
	networkSuite
}

func (s *DHTSuite) SetupSuite() {
	s.port = 1456
}

func (s *DHTSuite) TestDisabled() {
	m := s.newManager()
	defer m.Close()
	s.Nil(m.dht)
}

func (s *DHTSuite) TestTwoHosts() {
	conf := &Config{
		DHTEnabled: true,
	}
	m1 := s.newManagerWithBaseConfig(conf)
	defer m1.Close()

	conf.DHTBootstrapPeers = AddrInfoSlice{CalcAddress(m1)}
	m2 := s.newManagerWithBaseConfig(conf)
	defer m2.Close()

	WaitForPeer(s.T(), m1, m2.host.ID())
	WaitForPeer(s.T(), m2, m1.host.ID())
}

func TestDHT(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(DHTSuite))
}
