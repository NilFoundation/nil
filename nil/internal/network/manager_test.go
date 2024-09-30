package network

import (
	"context"
	"testing"

	"github.com/NilFoundation/nil/nil/common"
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

	conf = common.CopyPtr(conf)
	if conf.TcpPort == 0 {
		s.Require().Positive(s.port)
		s.port++
		conf.TcpPort = s.port
	}

	return NewTestManagerWithBaseConfig(s.T(), s.context, conf)
}

func (s *networkSuite) newManager() *Manager {
	s.T().Helper()

	return s.newManagerWithBaseConfig(&Config{})
}

type ManagerSuite struct {
	networkSuite
}

func (s *ManagerSuite) SetupSuite() {
	s.port = 1234
}

func (s *ManagerSuite) TestNewManager() {
	s.Run("EmptyConfig", func() {
		emptyConfig := &Config{}
		s.Require().False(emptyConfig.Enabled())

		_, err := NewManager(s.context, emptyConfig)
		s.Require().ErrorIs(err, ErrNetworkDisabled)
	})

	s.Run("NoPrivateKey", func() {
		_, err := NewManager(s.context, &Config{
			TcpPort: 1234,
		})
		s.Require().ErrorIs(err, ErrPrivateKeyMissing)
	})
}

func (s *ManagerSuite) TestPrivateKey() {
	privateKey, err := GeneratePrivateKey()
	s.Require().NoError(err)
	m := s.newManagerWithBaseConfig(&Config{
		PrivateKey: privateKey,
	})
	defer m.Close()

	s.Equal(privateKey, m.host.Peerstore().PrivKey(m.host.ID()))
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
		ConnectManagers(s.T(), m1, m2)
	})

	s.Run("Handle", func() {
		m2.SetRequestHandler(s.context, protocol, func(_ context.Context, msg []byte) ([]byte, error) {
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
