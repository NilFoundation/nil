package network

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"

	"github.com/NilFoundation/nil/nil/common/logging"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/rs/zerolog"
)

const subscriptionChannelSize = 100

type PubSub struct {
	impl *pubsub.PubSub

	mu     sync.Mutex
	topics map[string]*pubsub.Topic
	self   PeerID

	logger zerolog.Logger
}

type SubscriptionCounters struct {
	SkippedMessages atomic.Uint32
}

type Subscription struct {
	impl *pubsub.Subscription
	self PeerID

	logger   zerolog.Logger
	counters SubscriptionCounters
}

// newPubSub creates a new PubSub instance. It must be closed after use.
func newPubSub(ctx context.Context, h Host, logger zerolog.Logger) (*PubSub, error) {
	impl, err := pubsub.NewGossipSub(ctx, h)
	if err != nil {
		return nil, err
	}

	return &PubSub{
		impl:   impl,
		topics: make(map[string]*pubsub.Topic),
		self:   h.ID(),
		logger: logger.With().
			Str(logging.FieldComponent, "pub-sub").
			Logger(),
	}, nil
}

func (ps *PubSub) Close() error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	var errs []error
	for _, t := range ps.topics {
		if err := t.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

func (ps *PubSub) Topics() []string {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	topics := make([]string, 0, len(ps.topics))
	for topic := range ps.topics {
		topics = append(topics, topic)
	}

	return topics
}

// Publish publishes a message to the given topic.
func (ps *PubSub) Publish(ctx context.Context, topic string, data []byte) error {
	ps.logger.Trace().Str(logging.FieldTopic, topic).Msg("Publishing message...")

	t, err := ps.getTopic(topic)
	if err != nil {
		return err
	}

	return t.Publish(ctx, data)
}

// Subscribe subscribes to the given topic. The subscription must be closed after use.
func (ps *PubSub) Subscribe(topic string) (*Subscription, error) {
	logger := ps.logger.With().
		Str(logging.FieldComponent, "sub").
		Str(logging.FieldTopic, topic).
		Logger()

	t, err := ps.getTopic(topic)
	if err != nil {
		return nil, err
	}

	impl, err := t.Subscribe()
	if err != nil {
		return nil, err
	}

	logger.Debug().Msg("Subscribed to topic")
	return &Subscription{
		impl:   impl,
		self:   ps.self,
		logger: logger,
	}, nil
}

func (ps *PubSub) ListPeers(topic string) []PeerID {
	t, err := ps.getTopic(topic)
	if err != nil {
		return nil
	}

	return t.ListPeers()
}

func (ps *PubSub) getTopic(topic string) (*pubsub.Topic, error) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	if t, ok := ps.topics[topic]; ok {
		return t, nil
	}

	ps.logger.Debug().Str(logging.FieldTopic, topic).Msg("Joining topic...")

	t, err := ps.impl.Join(topic)
	if err != nil {
		return nil, err
	}

	ps.topics[topic] = t
	return t, nil
}

func (s *Subscription) Start(ctx context.Context) <-chan []byte {
	msgCh := make(chan []byte, subscriptionChannelSize)

	go func() {
		s.logger.Debug().Msg("Starting subscription loop...")

		for {
			msg, err := s.impl.Next(ctx)
			if err != nil {
				if ctx.Err() != nil {
					s.logger.Debug().Err(err).Msg("Closing subscription loop due to context cancellation")
					break
				}
				if errors.Is(err, pubsub.ErrSubscriptionCancelled) {
					s.logger.Debug().Err(err).Msg("Quitting subscription loop")
					break
				}
				s.logger.Error().Err(err).Msg("Error reading message")
				continue
			}

			if msg.ReceivedFrom == s.self {
				s.logger.Trace().Msg("Skip message from self")
				s.counters.SkippedMessages.Add(1)
				continue
			}

			s.logger.Trace().Msg("Received message")

			msgCh <- msg.Data
		}

		close(msgCh)

		s.logger.Debug().Msg("Subscription loop closed.")
	}()

	return msgCh
}

func (s *Subscription) Counters() *SubscriptionCounters {
	return &s.counters
}

func (s *Subscription) Close() {
	s.impl.Cancel()
}
