package network

import (
	"context"
	"testing"
	"time"

	"github.com/NilFoundation/nil/nil/common"
	cm "github.com/NilFoundation/nil/nil/internal/network/connection_manager"
	"github.com/jonboulle/clockwork"
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

	return NewTestManagerWithBaseConfig(s.context, s.T(), conf)
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

		_, err := NewManager(s.context, emptyConfig, nil)
		s.Require().ErrorIs(err, ErrNetworkDisabled)
	})

	s.Run("NoPrivateKey", func() {
		_, err := NewManager(s.context, &Config{
			TcpPort: 1234,
		}, nil)
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

type ConnectionManagerCheckParams struct {
	halfDecayTimeSeconds int
	forgetAfterTime      time.Duration
	timeoutToConnect     time.Duration
	timeoutToReconnect   time.Duration
	expectedToReconnect  bool
}

func (s *ManagerSuite) CheckConnectionManager(params ConnectionManagerCheckParams) {
	s.T().Helper()

	clock := clockwork.NewFakeClock()
	config := NewDefaultConfig()
	config.ConnectionManagerConfig.ForgetAfterTime = params.forgetAfterTime
	config.ConnectionManagerConfig.ReputationBanThreshold = //
		config.ConnectionManagerConfig.ReputationChangeSettings[cm.ReputationChangeInvalidBlockSignature] / 2
	config.ConnectionManagerConfig.DecayReputationPerSecondPercent = //
		cm.CalculateDecayPercent(params.halfDecayTimeSeconds, 0.5)
	cm.SetClock(config.ConnectionManagerConfig, clock)
	m1 := s.newManagerWithBaseConfig(config)

	defer m1.Close()
	m2 := s.newManager()
	defer m2.Close()

	peerReporter := TryGetPeerReputationTracker(m1)
	s.Require().NotNil(peerReporter)

	s.Run("Connect", func() {
		s.Require().Len(m1.host.Peerstore().Peers(), 1)
		s.Require().Empty(m1.host.Network().Peers())

		ConnectManagers(s.T(), m1, m2)

		s.Require().Len(m1.host.Peerstore().Peers(), 2)
		s.Require().Len(m1.host.Network().Peers(), 1)
	})

	s.Run("Report peer", func() {
		peerReporter.ReportPeer(m2.host.ID(), cm.ReputationChangeInvalidBlockSignature)

		s.Require().Len(m1.host.Peerstore().Peers(), 2)
		s.Require().Empty(m1.host.Network().Peers())
	})

	s.Run("Attempt to connect to banned peer", func() {
		clock.Advance(params.timeoutToConnect)

		ConnectManagers(s.T(), m1, m2)
		s.Require().Len(m1.host.Peerstore().Peers(), 2)
		s.Require().Empty(m1.host.Network().Peers())
	})

	s.Run("Attempt to reconnect to peer", func() {
		clock.Advance(params.timeoutToReconnect)

		ConnectManagers(s.T(), m1, m2)
		s.Require().Len(m1.host.Peerstore().Peers(), 2)
		if params.expectedToReconnect {
			s.Require().Len(m1.host.Network().Peers(), 1)
		} else {
			s.Require().Empty(m1.host.Network().Peers())
		}
	})
}

func (s *ManagerSuite) TestReconnectAfterReputationRecovery() {
	s.CheckConnectionManager(ConnectionManagerCheckParams{
		halfDecayTimeSeconds: 3,
		forgetAfterTime:      2 * time.Hour,
		timeoutToConnect:     2 * time.Second,
		timeoutToReconnect:   2 * time.Second,
		expectedToReconnect:  true,
	})
}

func (s *ManagerSuite) TestReconnectAfterForgettingPeer() {
	s.CheckConnectionManager(ConnectionManagerCheckParams{
		halfDecayTimeSeconds: 60,
		forgetAfterTime:      20 * time.Second,
		timeoutToConnect:     15 * time.Second,
		timeoutToReconnect:   30 * time.Second,
		expectedToReconnect:  true,
	})
}

func (s *ManagerSuite) TestDoNotReconnectWithoutForgettingPeer() {
	s.CheckConnectionManager(ConnectionManagerCheckParams{
		halfDecayTimeSeconds: 60,
		forgetAfterTime:      2 * time.Hour,
		timeoutToConnect:     15 * time.Second,
		timeoutToReconnect:   15 * time.Second,
		expectedToReconnect:  false,
	})
}

func TestManager(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(ManagerSuite))
}
