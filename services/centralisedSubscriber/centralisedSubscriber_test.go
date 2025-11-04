package centralisedSubscriber

import (
	"sync"
	"testing"

	"github.com/alphadose/haxmap"
	"github.com/stretchr/testify/assert"
)

// Helper to get internal map safely for testing
func getChannelCount(s *centralisedSubscriber) int {
	count := 0
	s.channelsTracker.ForEach(func(_ string, _ *haxmap.Map[string, struct{}]) bool {
		count++
		return true
	})
	return count
}

func getClientCountForChannel(s *centralisedSubscriber, channel string) int {
	subMap, ok := s.channelsTracker.Get(channel)
	if !ok {
		return 0
	}
	return int(subMap.Len())
}

func TestSubscribeAndUnsubscribe(t *testing.T) {
	s := New().(*centralisedSubscriber)

	// Subscribe multiple clients
	s.Subscribe("client-1", "sports")
	s.Subscribe("client-2", "sports")

	assert.Equal(t, 1, getChannelCount(s))
	assert.Equal(t, 2, getClientCountForChannel(s, "sports"))

	// Unsubscribe one
	s.UnSubscribe("client-1", "sports")
	assert.Equal(t, 1, getClientCountForChannel(s, "sports"))

	// Unsubscribe last client -> should delete channel
	s.UnSubscribe("client-2", "sports")
	assert.Equal(t, 0, getChannelCount(s))
}

func TestUnsubscribeAll(t *testing.T) {
	s := New().(*centralisedSubscriber)

	s.Subscribe("c1", "news")
	s.Subscribe("c1", "sports")
	s.Subscribe("c2", "sports")

	assert.Equal(t, 2, getChannelCount(s))

	s.UnsubscribeAll("c1")

	// c1 should be removed from both channels
	assert.Equal(t, 1, getClientCountForChannel(s, "sports"))
	assert.Equal(t, 0, getClientCountForChannel(s, "news")) // should remove channel as empty

	// channel "news" should be deleted, only "sports" remains
	assert.Equal(t, 1, getChannelCount(s))
}

func TestConcurrentSubscribeUnsubscribe(t *testing.T) {
	s := New().(*centralisedSubscriber)

	var wg sync.WaitGroup
	numClients := 100
	channel := "tech"

	// Concurrent subscribes
	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			s.Subscribe(
				string(rune(id)), // fake ID
				channel,
			)
		}(i)
	}
	wg.Wait()

	assert.Equal(t, 1, getChannelCount(s))
	assert.Equal(t, numClients, getClientCountForChannel(s, channel))

	// Concurrent unsubscribe all
	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			s.UnSubscribe(string(rune(id)), channel)
		}(i)
	}
	wg.Wait()

	assert.Equal(t, 0, getClientCountForChannel(s, channel))
	assert.Equal(t, 0, getChannelCount(s))
}

func TestUnsubscribeNonExistent(t *testing.T) {
	s := New().(*centralisedSubscriber)

	// Should not panic
	s.UnSubscribe("unknown-client", "unknown-channel")
	assert.Equal(t, 0, getChannelCount(s))
}
