package network

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"
)

type networkSuite struct {
	suite.Suite

	context   context.Context
	ctxCancel context.CancelFunc

	port int
}

func (s *networkSuite) SetupTest() {
	s.context, s.ctxCancel = context.WithCancel(context.Background())
}

func (s *networkSuite) TearDownTest() {
	s.ctxCancel()
}

func (s *networkSuite) newManagerWithBaseConfig(conf *Config) *Manager {
	s.T().Helper()

	if conf.IPV4Address == "" {
		conf.IPV4Address = "127.0.0.1"
	}
	if conf.TcpPort == 0 {
		s.Require().Positive(s.port)
		s.port++
		conf.TcpPort = s.port
	}

	m, err := NewManager(s.context, conf)
	s.Require().NoError(err)
	return m
}

func (s *networkSuite) newManager() *Manager {
	s.T().Helper()

	return s.newManagerWithBaseConfig(&Config{})
}

func (s *networkSuite) connectManagers(m1, m2 *Manager) {
	s.T().Helper()

	id, err := m1.Connect(s.context, m2.host.Addrs()[0].String()+"/p2p/"+m2.host.ID().String())
	s.Require().NoError(err)
	s.Equal(m2.host.ID(), id)
}

type ManagerSuite struct {
	networkSuite
}

func (s *ManagerSuite) SetupSuite() {
	s.port = 1234
}

func (s *ManagerSuite) TestNewManager() {
	s.Run("EmptyConfig", func() {
		emptyConfig := new(Config)
		s.Require().False(emptyConfig.Enabled())
		s.Require().Panics(func() {
			_, _ = NewManager(s.context, emptyConfig)
		})
	})
}

func (s *ManagerSuite) TestReqResp() {
	m1 := s.newManager()
	defer m1.Close()
	m2 := s.newManager()
	defer m2.Close()

	const protocol = "test-p"
	request := []byte("hello")
	response := []byte("world")

	s.Run("Connect", func() {
		s.connectManagers(m1, m2)
	})

	s.Run("Handle", func() {
		m2.SetRequestHandler(protocol, func(_ context.Context, msg []byte) ([]byte, error) {
			s.Equal(request, msg)
			return response, nil
		})
	})

	s.Run("Request", func() {
		resp, err := m1.SendRequestAndGetResponse(s.context, m2.host.ID(), protocol, request)
		s.Require().NoError(err)
		s.Equal(response, resp)
	})
}

func TestManager(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(ManagerSuite))
}
