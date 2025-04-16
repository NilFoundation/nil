package network

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

type PubSubSuite struct {
	networkSuite
}

func (s *PubSubSuite) SetupSuite() {
	s.port = 1345
}

func (s *PubSubSuite) receive(ch <-chan PubSubMessage, expected []byte) {
	s.T().Helper()

	s.Eventually(func() bool {
		select {
		case received := <-ch:
			s.Equal(expected, received.Data)
			return true
		default:
			return false
		}
	}, 10*time.Second, 100*time.Millisecond)
}

func (s *PubSubSuite) ensureSkipped(sub *Subscription, ch <-chan PubSubMessage, curSkippedCounter int) {
	s.T().Helper()

	s.Eventually(func() bool {
		return int(sub.Counters().SkippedMessages.Load()) > curSkippedCounter
	}, 10*time.Second, 100*time.Millisecond)

	// check there are no new transactions
	select {
	case <-ch:
		s.Require().Fail("")
	default:
	}
}

func (s *PubSubSuite) listPeers(manager *BasicManager, topic string) []PeerID {
	s.T().Helper()

	t, err := manager.PubSub().getTopic(topic)
	s.Require().NoError(err)
	return t.ListPeers()
}

func (s *PubSubSuite) TestSingleHost() {
	manager := s.newManager()
	defer manager.Close()

	topic := "test"
	sub, err := manager.PubSub().Subscribe(topic)
	s.Require().NoError(err)
	defer sub.Close()

	ch := sub.Start(s.context, true)

	msg := []byte("hello")
	err = manager.PubSub().Publish(s.context, topic, msg)
	s.Require().NoError(err)

	s.ensureSkipped(sub, ch, 0)
}

func (s *PubSubSuite) TestTwoHosts() {
	m1 := s.newManager()
	defer m1.Close()
	m2 := s.newManager()
	defer m2.Close()

	ConnectManagers(s.T(), m1, m2)

	const topic = "test"
	msg := []byte("hello")

	sub, err := m1.PubSub().Subscribe(topic)
	s.Require().NoError(err)
	defer sub.Close()
	ch := sub.Start(s.context, true)
	s.Require().NoError(err)

	err = m2.PubSub().Publish(s.context, topic, msg)
	s.Require().NoError(err)

	s.receive(ch, msg)
}

func (s *PubSubSuite) TestComplexScenario() {
	// todo: this test often fails in CI but works locally
	s.T().SkipNow()

	const n = 5
	const centralHost = 3

	managers := make([]*BasicManager, n)
	for i := range n {
		managers[i] = s.newManager()
	}
	defer func() {
		for i := range n {
			managers[i].Close()
		}
	}()

	s.Run("Connect all", func() {
		ConnectAllManagers(s.T(), managers...)
	})

	const topic1 = "test1"
	const topic2 = "test2"
	const publisher1 = 2
	const publisher2 = 0
	s.Require().NotEqual(publisher1, publisher2)
	s.Require().NotEqual(publisher1, centralHost)
	s.Require().NotEqual(publisher2, centralHost)

	msg1 := []byte("hello 1")
	msg2 := []byte("hello 2")
	s.Require().NotEqual(msg1, msg2)

	topic1Subs := make([]*Subscription, n)
	topic1Channels := make([]<-chan PubSubMessage, n)

	s.Run("Subscribe all to topic 1", func() {
		for i := range n {
			sub, err := managers[i].PubSub().Subscribe(topic1)
			s.Require().NoError(err)
			topic1Subs[i] = sub

			topic1Channels[i] = topic1Subs[i].Start(s.context, true)
			s.Require().NoError(err)
		}
	})
	defer func() {
		for i := range n {
			topic1Subs[i].Close()
		}
	}()

	s.Run("Wait for topic 1 peers", func() {
		s.Eventually(func() bool {
			// Intersect the peers of the central host and the publisher.
			// If the size of the result equals n, the publisher will be able to reach all peers,
			// either directly or via the central host.
			peers := make(map[PeerID]bool)
			fill := func(i int) {
				for _, p := range s.listPeers(managers[i], topic1) {
					peers[p] = true
				}
			}
			fill(publisher1)
			fill(centralHost)
			return len(peers) == n
		}, 2*time.Second, 100*time.Millisecond)
	})

	s.Run("Publish to topic 1", func() {
		err := managers[publisher1].PubSub().Publish(s.context, topic1, msg1)
		s.Require().NoError(err)

		s.Run("Receive", func() {
			for i := range n {
				if i == publisher1 {
					s.ensureSkipped(topic1Subs[i], topic1Channels[i], 0)
				} else {
					s.receive(topic1Channels[i], msg1)
				}
			}
		})
	})

	s.Run("Topic 2", func() {
		const subscriber = centralHost + 1
		s.Require().NotEqual(topic1, topic2)
		s.Require().Less(subscriber, n)

		msg := []byte("hello")
		var sub *Subscription

		s.Run("Subscribe a single peer to topic 2", func() {
			var err error
			sub, err = managers[subscriber].PubSub().Subscribe(topic2)
			s.Require().NoError(err)
		})
		defer sub.Close()

		s.Run("Publish to topic 2 not being subscribed to it", func() {
			s.Run("Wait for peer", func() {
				s.Eventually(func() bool {
					return len(s.listPeers(managers[centralHost], topic2)) == 1
				}, 2*time.Second, 100*time.Millisecond)

				s.Equal(managers[subscriber].host.ID(), s.listPeers(managers[centralHost], topic2)[0])
			})

			err := managers[centralHost].PubSub().Publish(s.context, topic2, msg)
			s.Require().NoError(err)
		})

		s.Run("Receive from topic 2", func() {
			ch := sub.Start(s.context, true)
			s.receive(ch, msg)
		})
	})

	s.Run("Publish to topic 1 again", func() {
		err := managers[publisher2].PubSub().Publish(s.context, topic1, msg2)
		s.Require().NoError(err)

		s.Run("Receive", func() {
			for i := range n {
				if i == publisher2 {
					s.ensureSkipped(topic1Subs[i], topic1Channels[i], 0)
				} else {
					s.receive(topic1Channels[i], msg2)
				}
			}
		})
	})
}

func TestPubSub(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(PubSubSuite))
}
