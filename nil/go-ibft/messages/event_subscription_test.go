package messages

import (
	"testing"

	"github.com/NilFoundation/nil/nil/go-ibft/messages/proto"
	"github.com/stretchr/testify/assert"
)

func TestEventSubscription_EventSupported(t *testing.T) {
	t.Parallel()

	type signalDetails struct {
		view          *proto.View
		totalMessages int
		messageType   proto.MessageType
	}

	commonDetails := SubscriptionDetails{
		MessageType: proto.MessageType_PREPARE,
		View: &proto.View{
			Height: 0,
			Round:  0,
		},
		MinNumMessages: 10,
	}

	testTable := []struct {
		event               signalDetails
		name                string
		subscriptionDetails SubscriptionDetails
		shouldSupport       bool
	}{
		{
			signalDetails{
				commonDetails.View,
				commonDetails.MinNumMessages,
				commonDetails.MessageType,
			},
			"Same signal as subscription",
			commonDetails,
			true,
		},
		{
			signalDetails{
				&proto.View{
					Height: commonDetails.View.Height,
					Round:  commonDetails.View.Round + 1,
				},
				commonDetails.MinNumMessages,
				commonDetails.MessageType,
			},
			"Message round > round than subscription (supported)",
			SubscriptionDetails{
				MessageType:    commonDetails.MessageType,
				View:           commonDetails.View,
				MinNumMessages: commonDetails.MinNumMessages,
				HasMinRound:    true,
			},
			true,
		},
		{
			signalDetails{
				commonDetails.View,
				commonDetails.MinNumMessages,
				commonDetails.MessageType,
			},
			"Message round == round than subscription (supported)",
			SubscriptionDetails{
				MessageType:    commonDetails.MessageType,
				View:           commonDetails.View,
				MinNumMessages: commonDetails.MinNumMessages,
				HasMinRound:    true,
			},
			true,
		},
		{
			signalDetails{
				&proto.View{
					Height: commonDetails.View.Height,
					Round:  commonDetails.View.Round + 1,
				},
				commonDetails.MinNumMessages,
				commonDetails.MessageType,
			},
			"Message round > round than subscription (not supported)",
			commonDetails,
			false,
		},
		{
			signalDetails{
				&proto.View{
					Height: commonDetails.View.Height,
					Round:  commonDetails.View.Round + 10 - 1,
				},
				commonDetails.MinNumMessages,
				commonDetails.MessageType,
			},
			"Message round < round than subscription (not supported)",
			SubscriptionDetails{
				MessageType: commonDetails.MessageType,
				View: &proto.View{
					Height: commonDetails.View.Height,
					Round:  commonDetails.View.Round + 10,
				},
				MinNumMessages: commonDetails.MinNumMessages,
				HasMinRound:    true,
			},
			false,
		},
		{
			signalDetails{
				commonDetails.View,
				commonDetails.MinNumMessages - 1,
				commonDetails.MessageType,
			},
			"Lower number of messages",
			commonDetails,
			false,
		},
		{
			signalDetails{
				commonDetails.View,
				commonDetails.MinNumMessages,
				proto.MessageType_COMMIT,
			},
			"Invalid message type",
			commonDetails,
			false,
		},
		{
			signalDetails{
				&proto.View{
					Height: commonDetails.View.Height + 1,
					Round:  commonDetails.View.Round,
				},
				commonDetails.MinNumMessages,
				commonDetails.MessageType,
			},
			"Invalid message height",
			commonDetails,
			false,
		},
	}

	for _, testCase := range testTable {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			subscription := &eventSubscription{
				details:  testCase.subscriptionDetails,
				outputCh: make(chan uint64, 1),
				notifyCh: make(chan uint64, 1),
				doneCh:   make(chan struct{}),
			}

			t.Cleanup(func() {
				subscription.close()
			})

			event := testCase.event

			assert.Equal(
				t,
				testCase.shouldSupport,
				subscription.eventSupported(
					event.messageType,
					event.view,
					event.totalMessages,
				),
			)
		})
	}
}
