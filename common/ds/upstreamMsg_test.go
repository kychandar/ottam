package ds

import (
	"testing"

	"github.com/kychandar/ottam/common"
	"github.com/stretchr/testify/assert"
)

func TestClientMessage(t *testing.T) {
	t.Run("create client message", func(t *testing.T) {
		msg := ClientMessage{
			Version:               1,
			Id:                    "client-msg-123",
			IsControlPlaneMessage: false,
			Payload:               "test payload",
		}

		assert.Equal(t, uint32(1), msg.Version)
		assert.Equal(t, "client-msg-123", msg.Id)
		assert.False(t, msg.IsControlPlaneMessage)
		assert.Equal(t, "test payload", msg.Payload)
	})

	t.Run("client message with control plane flag", func(t *testing.T) {
		msg := ClientMessage{
			Version:               1,
			Id:                    "ctrl-msg-456",
			IsControlPlaneMessage: true,
			Payload:               map[string]string{"key": "value"},
		}

		assert.True(t, msg.IsControlPlaneMessage)
		payload, ok := msg.Payload.(map[string]string)
		assert.True(t, ok)
		assert.Equal(t, "value", payload["key"])
	})

	t.Run("client message with nil payload", func(t *testing.T) {
		msg := ClientMessage{
			Version: 1,
			Id:      "msg-789",
			Payload: nil,
		}

		assert.Nil(t, msg.Payload)
	})

	t.Run("client message zero value", func(t *testing.T) {
		var msg ClientMessage
		assert.Equal(t, uint32(0), msg.Version)
		assert.Equal(t, "", msg.Id)
		assert.False(t, msg.IsControlPlaneMessage)
		assert.Nil(t, msg.Payload)
	})

	t.Run("client message with various payload types", func(t *testing.T) {
		testCases := []struct {
			name    string
			payload any
		}{
			{"string payload", "test string"},
			{"int payload", 12345},
			{"float payload", 3.14159},
			{"bool payload", true},
			{"slice payload", []string{"a", "b", "c"}},
			{"map payload", map[string]int{"x": 1, "y": 2}},
			{"struct payload", SubscriptionPayload{ChannelName: "test"}},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				msg := ClientMessage{
					Version: 1,
					Id:      "msg",
					Payload: tc.payload,
				}
				assert.Equal(t, tc.payload, msg.Payload)
			})
		}
	})
}

func TestControlPlaneOp(t *testing.T) {
	t.Run("control plane operation values", func(t *testing.T) {
		assert.Equal(t, ControlPlaneOp(0), StartSubscription)
		assert.Equal(t, ControlPlaneOp(1), StopSubscription)
	})

	t.Run("control plane operations are distinct", func(t *testing.T) {
		assert.NotEqual(t, StartSubscription, StopSubscription)
	})

	t.Run("control plane operation type", func(t *testing.T) {
		var op ControlPlaneOp
		op = StartSubscription
		assert.Equal(t, ControlPlaneOp(0), op)

		op = StopSubscription
		assert.Equal(t, ControlPlaneOp(1), op)
	})
}

func TestControlPlaneMessage(t *testing.T) {
	t.Run("create control plane message for start subscription", func(t *testing.T) {
		msg := ControlPlaneMessage{
			Version:        1,
			ControlPlaneOp: StartSubscription,
			Payload: SubscriptionPayload{
				ChannelName: "test-channel",
			},
		}

		assert.Equal(t, uint32(1), msg.Version)
		assert.Equal(t, StartSubscription, msg.ControlPlaneOp)
		
		payload, ok := msg.Payload.(SubscriptionPayload)
		assert.True(t, ok)
		assert.Equal(t, common.ChannelName("test-channel"), payload.ChannelName)
	})

	t.Run("create control plane message for stop subscription", func(t *testing.T) {
		msg := ControlPlaneMessage{
			Version:        1,
			ControlPlaneOp: StopSubscription,
			Payload: SubscriptionPayload{
				ChannelName: "stop-channel",
			},
		}

		assert.Equal(t, StopSubscription, msg.ControlPlaneOp)
		
		payload, ok := msg.Payload.(SubscriptionPayload)
		assert.True(t, ok)
		assert.Equal(t, common.ChannelName("stop-channel"), payload.ChannelName)
	})

	t.Run("control plane message with nil payload", func(t *testing.T) {
		msg := ControlPlaneMessage{
			Version:        1,
			ControlPlaneOp: StartSubscription,
			Payload:        nil,
		}

		assert.Nil(t, msg.Payload)
	})

	t.Run("control plane message zero value", func(t *testing.T) {
		var msg ControlPlaneMessage
		assert.Equal(t, uint32(0), msg.Version)
		assert.Equal(t, StartSubscription, msg.ControlPlaneOp) // default is 0
		assert.Nil(t, msg.Payload)
	})

	t.Run("control plane message with different versions", func(t *testing.T) {
		versions := []uint32{1, 2, 3, 10, 100}
		for _, version := range versions {
			msg := ControlPlaneMessage{
				Version:        version,
				ControlPlaneOp: StartSubscription,
			}
			assert.Equal(t, version, msg.Version)
		}
	})
}

func TestSubscriptionPayload(t *testing.T) {
	t.Run("create subscription payload", func(t *testing.T) {
		payload := SubscriptionPayload{
			ChannelName: "my-channel",
		}

		assert.Equal(t, common.ChannelName("my-channel"), payload.ChannelName)
	})

	t.Run("subscription payload with empty channel", func(t *testing.T) {
		payload := SubscriptionPayload{
			ChannelName: "",
		}

		assert.Equal(t, common.ChannelName(""), payload.ChannelName)
	})

	t.Run("subscription payload zero value", func(t *testing.T) {
		var payload SubscriptionPayload
		assert.Equal(t, common.ChannelName(""), payload.ChannelName)
	})

	t.Run("subscription payload with special channel names", func(t *testing.T) {
		testChannels := []common.ChannelName{
			"simple",
			"with-dashes",
			"with_underscores",
			"with.dots",
			"with:colons",
			"with/slashes",
			">",
			"*",
			"channel.>",
		}

		for _, channel := range testChannels {
			payload := SubscriptionPayload{ChannelName: channel}
			assert.Equal(t, channel, payload.ChannelName)
		}
	})
}

func TestUpstreamMessageIntegration(t *testing.T) {
	t.Run("complete workflow - start subscription", func(t *testing.T) {
		// Create a client message for starting subscription
		clientMsg := ClientMessage{
			Version:               1,
			Id:                    "request-123",
			IsControlPlaneMessage: true,
			Payload: ControlPlaneMessage{
				Version:        1,
				ControlPlaneOp: StartSubscription,
				Payload: SubscriptionPayload{
					ChannelName: "chat-room-1",
				},
			},
		}

		assert.True(t, clientMsg.IsControlPlaneMessage)
		
		ctrlMsg, ok := clientMsg.Payload.(ControlPlaneMessage)
		assert.True(t, ok)
		assert.Equal(t, StartSubscription, ctrlMsg.ControlPlaneOp)
		
		subPayload, ok := ctrlMsg.Payload.(SubscriptionPayload)
		assert.True(t, ok)
		assert.Equal(t, common.ChannelName("chat-room-1"), subPayload.ChannelName)
	})

	t.Run("complete workflow - stop subscription", func(t *testing.T) {
		// Create a client message for stopping subscription
		clientMsg := ClientMessage{
			Version:               1,
			Id:                    "request-456",
			IsControlPlaneMessage: true,
			Payload: ControlPlaneMessage{
				Version:        1,
				ControlPlaneOp: StopSubscription,
				Payload: SubscriptionPayload{
					ChannelName: "chat-room-2",
				},
			},
		}

		assert.True(t, clientMsg.IsControlPlaneMessage)
		
		ctrlMsg, ok := clientMsg.Payload.(ControlPlaneMessage)
		assert.True(t, ok)
		assert.Equal(t, StopSubscription, ctrlMsg.ControlPlaneOp)
		
		subPayload, ok := ctrlMsg.Payload.(SubscriptionPayload)
		assert.True(t, ok)
		assert.Equal(t, common.ChannelName("chat-room-2"), subPayload.ChannelName)
	})

	t.Run("non-control plane message", func(t *testing.T) {
		clientMsg := ClientMessage{
			Version:               1,
			Id:                    "data-msg-789",
			IsControlPlaneMessage: false,
			Payload:               []byte("regular data message"),
		}

		assert.False(t, clientMsg.IsControlPlaneMessage)
		dataPayload, ok := clientMsg.Payload.([]byte)
		assert.True(t, ok)
		assert.Equal(t, []byte("regular data message"), dataPayload)
	})
}

func TestControlPlaneOpSwitch(t *testing.T) {
	t.Run("switch on control plane operations", func(t *testing.T) {
		operations := []ControlPlaneOp{StartSubscription, StopSubscription}
		
		for _, op := range operations {
			switch op {
			case StartSubscription:
				assert.Equal(t, StartSubscription, op)
			case StopSubscription:
				assert.Equal(t, StopSubscription, op)
			default:
				t.Fatal("unexpected operation")
			}
		}
	})
}

func TestMessageVersioning(t *testing.T) {
	t.Run("client message versioning", func(t *testing.T) {
		msg := ClientMessage{Version: 2}
		assert.Equal(t, uint32(2), msg.Version)
	})

	t.Run("control plane message versioning", func(t *testing.T) {
		msg := ControlPlaneMessage{Version: 3}
		assert.Equal(t, uint32(3), msg.Version)
	})

	t.Run("version compatibility", func(t *testing.T) {
		clientMsg := ClientMessage{
			Version: 1,
			Payload: ControlPlaneMessage{
				Version: 1,
			},
		}

		assert.Equal(t, clientMsg.Version, clientMsg.Payload.(ControlPlaneMessage).Version)
	})
}

