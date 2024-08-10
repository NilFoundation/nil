package network

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

type ManagerSuite struct {
	suite.Suite

	context   context.Context
	ctxCancel context.CancelFunc

	port int
}

func (s *ManagerSuite) SetupSuite() {
	s.port = 1234
}

func (s *ManagerSuite) SetupTest() {
	s.context, s.ctxCancel = context.WithCancel(context.Background())
}

func (s *ManagerSuite) TearDownTest() {
	s.ctxCancel()
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

func (s *ManagerSuite) newManagerWithBaseConfig(conf *Config) *Manager {
	s.T().Helper()

	s.port++
	conf.IPV4Address = "127.0.0.1"
	conf.TcpPort = s.port

	m, err := NewManager(s.context, conf)
	s.Require().NoError(err)
	return m
}

func (s *ManagerSuite) newManager() *Manager {
	s.T().Helper()

	return s.newManagerWithBaseConfig(&Config{})
}

func (s *ManagerSuite) TestPubSub() {
	m1 := s.newManager()
	defer m1.Close()
	m2 := s.newManager()
	defer m2.Close()

	topic := "test"
	msg := []byte("hello")
	var sub *Subscription

	s.Run("Connect", func() {
		id, err := m1.Connect(s.context, m2.host.Addrs()[0].String()+"/p2p/"+m2.host.ID().String())
		s.Require().NoError(err)
		s.Equal(m2.host.ID(), id)
	})

	s.Run("Subscribe", func() {
		var err error
		sub, err = m1.PubSub().Subscribe(topic)
		s.Require().NoError(err)
	})
	defer sub.Close()

	s.Run("Publish", func() {
		err := m2.PubSub().Publish(s.context, topic, msg)
		s.Require().NoError(err)
	})

	s.Run("Receive", func() {
		ch, err := sub.Start(s.context)
		s.Require().NoError(err)
		s.Eventually(func() bool {
			select {
			case received := <-ch:
				s.Equal(msg, received)
				return true
			default:
				return false
			}
		}, time.Second, 100*time.Millisecond)
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
		id, err := m1.Connect(s.context, m2.host.Addrs()[0].String()+"/p2p/"+m2.host.ID().String())
		s.Require().NoError(err)
		s.Equal(m2.host.ID(), id)
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
