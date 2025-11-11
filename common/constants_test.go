package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestChannelSubscCacheKeyFormat(t *testing.T) {
	tests := []struct {
		name     string
		channel  string
		expected string
	}{
		{
			name:     "simple channel name",
			channel:  "chat-room-1",
			expected: "0-chat-room-1",
		},
		{
			name:     "empty channel",
			channel:  "",
			expected: "0-",
		},
		{
			name:     "channel with special chars",
			channel:  "user:123:notifications",
			expected: "0-user:123:notifications",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ChannelSubscCacheKeyFormat(tt.channel)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestServerSubjFormat(t *testing.T) {
	tests := []struct {
		name     string
		server   string
		expected string
	}{
		{
			name:     "simple server name",
			server:   "server-1",
			expected: "server-server-1",
		},
		{
			name:     "empty server",
			server:   "",
			expected: "server-",
		},
		{
			name:     "server with UUID",
			server:   "abc-123-def-456",
			expected: "server-abc-123-def-456",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ServerSubjFormat(tt.server)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestChannelSubjFormat(t *testing.T) {
	tests := []struct {
		name     string
		channel  string
		expected string
	}{
		{
			name:     "simple channel",
			channel:  "general",
			expected: "ch.general",
		},
		{
			name:     "empty channel",
			channel:  "",
			expected: "ch.",
		},
		{
			name:     "wildcard channel",
			channel:  ">",
			expected: "ch.>",
		},
		{
			name:     "hierarchical channel",
			channel:  "user.123.notifications",
			expected: "ch.user.123.notifications",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ChannelSubjFormat(tt.channel)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConsumerNameForNode(t *testing.T) {
	tests := []struct {
		name     string
		hostName string
		expected string
	}{
		{
			name:     "simple hostname",
			hostName: "node-1",
			expected: "ottam-node-node-1",
		},
		{
			name:     "empty hostname",
			hostName: "",
			expected: "ottam-node-",
		},
		{
			name:     "hostname with domain",
			hostName: "ottam-pod-abc123.cluster.local",
			expected: "ottam-node-ottam-pod-abc123.cluster.local",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConsumerNameForNode(tt.hostName)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConstants(t *testing.T) {
	t.Run("OttamServerStreamName", func(t *testing.T) {
		assert.Equal(t, "ottam-server", OttamServerStreamName)
	})

	t.Run("ConsumerNameFormat", func(t *testing.T) {
		assert.Equal(t, "ottam-node-%s", ConsumerNameFormat)
	})

	t.Run("StreamNamePublisher", func(t *testing.T) {
		assert.Equal(t, "publisher", StreamNamePublisher)
	})

	t.Run("ConsumerNameCentProcessor", func(t *testing.T) {
		assert.Equal(t, "centProcessor", ConsumerNameCentProcessor)
	})
}

func TestCachePrefix(t *testing.T) {
	t.Run("channelSubscription value", func(t *testing.T) {
		assert.Equal(t, CachePrefix(0), channelSubscription)
	})
}

func TestTypeAliases(t *testing.T) {
	t.Run("ChannelName type", func(t *testing.T) {
		var cn ChannelName = "test-channel"
		assert.Equal(t, "test-channel", string(cn))
	})

	t.Run("NodeID type", func(t *testing.T) {
		var nid NodeID = "node-123"
		assert.Equal(t, "node-123", string(nid))
	})
}

