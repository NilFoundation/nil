package network

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"

	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/telemetry"
	"github.com/NilFoundation/nil/nil/internal/telemetry/telattr"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/rs/zerolog"
)

const subscriptionChannelSize = 100

type PubSub struct {
	impl   *pubsub.PubSub // +checklocksignore: mu is not required, it just happens to be held always.
	prefix string

	mu     sync.Mutex
	topics map[string]*pubsub.Topic // +checklocks:mu
	self   PeerID

	meter         telemetry.Meter
	published     telemetry.Counter
	publishedSize telemetry.Counter

	logger zerolog.Logger
}

type SubscriptionCounters struct {
	SkippedMessages atomic.Uint32
}

type Subscription struct {
	impl *pubsub.Subscription
	self PeerID

	received     telemetry.Counter
	receivedSize telemetry.Counter
	logger       zerolog.Logger
	counters     SubscriptionCounters
}

// newPubSub creates a new PubSub instance. It must be closed after use.
func newPubSub(ctx context.Context, h Host, conf *Config, logger zerolog.Logger) (*PubSub, error) {
	impl, err := pubsub.NewGossipSub(ctx, h)
	if err != nil {
		return nil, err
	}

	meter := telemetry.NewMeter("github.com/NilFoundation/nil/nil/internal/network/pubsub")
	published, err := meter.Int64Counter("published_messages")
	if err != nil {
		return nil, err
	}
	publishedSize, err := meter.Int64Counter("published_messages_size")
	if err != nil {
		return nil, err
	}

	return &PubSub{
		prefix:        conf.Prefix,
		impl:          impl,
		topics:        make(map[string]*pubsub.Topic),
		self:          h.ID(),
		meter:         meter,
		published:     published,
		publishedSize: publishedSize,
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

func (ps *PubSub) withNetworkPrefix(prefix string) string {
	return ps.prefix + prefix
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

	if err := t.Publish(ctx, data); err != nil {
		return err
	}

	attrs := telattr.With(telattr.Topic(topic), telattr.P2PIdentity(ps.self))
	ps.published.Add(ctx, 1, attrs)
	ps.publishedSize.Add(ctx, int64(len(data)), attrs)

	return nil
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

	received, err := ps.meter.Int64Counter("received_messages")
	if err != nil {
		return nil, err
	}
	receivedSize, err := ps.meter.Int64Counter("received_messages_size")
	if err != nil {
		return nil, err
	}

	logger.Debug().Msg("Subscribed to topic")
	return &Subscription{
		impl:         impl,
		self:         ps.self,
		received:     received,
		receivedSize: receivedSize,
		logger:       logger,
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
	topic = ps.withNetworkPrefix(topic)

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

func (s *Subscription) Start(ctx context.Context, skipSelfMessages bool) <-chan []byte {
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

			if skipSelfMessages && msg.ReceivedFrom == s.self {
				s.logger.Trace().Msg("Skip message from self")
				s.counters.SkippedMessages.Add(1)
				continue
			}

			attrs := telattr.With(telattr.Topic(s.impl.Topic()), telattr.P2PIdentity(s.self))
			s.received.Add(ctx, 1, attrs)
			s.receivedSize.Add(ctx, int64(len(msg.Data)), attrs)
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
