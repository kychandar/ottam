package pubSubProvider

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/kychandar/ottam/services"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func runEmbeddedNATSServer(t *testing.T) *server.Server {
	opts := &server.Options{
		Port:      -1, // random available port
		JetStream: true,
	}
	s, err := server.NewServer(opts)
	require.NoError(t, err, "failed to start embedded NATS")
	
	go s.Start()
	require.True(t, s.ReadyForConnections(2*time.Second), "nats-server not ready")
	
	t.Cleanup(func() {
		s.Shutdown()
		s.WaitForShutdown()
	})
	return s
}

func newTestPubSubProvider(t *testing.T) services.PubSubProvider {
	s := runEmbeddedNATSServer(t)
	url := fmt.Sprintf("nats://%s", s.Addr().String())

	pubsub, err := NewNatsPubSub(url)
	require.NoError(t, err, "failed to create NatsPubSub")
	
	t.Cleanup(func() {
		pubsub.Close()
	})
	
	return pubsub
}

func TestNewNatsPubSub_Success(t *testing.T) {
	s := runEmbeddedNATSServer(t)
	url := fmt.Sprintf("nats://%s", s.Addr().String())

	pubsub, err := NewNatsPubSub(url)
	
	assert.NoError(t, err)
	assert.NotNil(t, pubsub)
	
	assert.NoError(t, pubsub.Close())
}

func TestNewNatsPubSub_InvalidURL(t *testing.T) {
	pubsub, err := NewNatsPubSub("nats://invalid-host:4222")
	
	assert.Error(t, err)
	assert.Nil(t, pubsub)
}

func TestCreateOrUpdateStream_NewStream(t *testing.T) {
	pubsub := newTestPubSubProvider(t)
	ctx := context.Background()

	streamName := uuid.New().String()
	subjects := []string{"test.subject.>"}

	err := pubsub.CreateOrUpdateStream(ctx, streamName, subjects)
	
	assert.NoError(t, err)
}

func TestCreateOrUpdateStream_UpdateExisting(t *testing.T) {
	pubsub := newTestPubSubProvider(t)
	ctx := context.Background()

	streamName := uuid.New().String()
	subjects1 := []string{"test.subject.1"}

	// Create stream
	err := pubsub.CreateOrUpdateStream(ctx, streamName, subjects1)
	require.NoError(t, err)

	// Update stream with new subjects
	subjects2 := []string{"test.subject.2"}
	err = pubsub.CreateOrUpdateStream(ctx, streamName, subjects2)
	
	assert.NoError(t, err)
}

func TestCreateOrUpdateStream_ContextCancellation(t *testing.T) {
	pubsub := newTestPubSubProvider(t)
	
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	streamName := uuid.New().String()
	subjects := []string{"test.subject"}

	err := pubsub.CreateOrUpdateStream(ctx, streamName, subjects)
	
	assert.Error(t, err)
}

func TestPublish_Success(t *testing.T) {
	pubsub := newTestPubSubProvider(t)
	ctx := context.Background()

	streamName := uuid.New().String()
	subject := "test.publish"
	
	// Create stream first
	err := pubsub.CreateOrUpdateStream(ctx, streamName, []string{subject + ".>"})
	require.NoError(t, err)

	// Publish message
	data := []byte("hello world")
	err = pubsub.Publish(ctx, subject+".1", data)
	
	assert.NoError(t, err)
}

func TestPublish_WithoutStream(t *testing.T) {
	pubsub := newTestPubSubProvider(t)
	ctx := context.Background()

	// Publish without creating stream first
	data := []byte("test data")
	err := pubsub.Publish(ctx, "nonexistent.subject", data)
	
	// Should get an error about no stream matching
	assert.Error(t, err)
}

func TestCreateOrUpdateConsumer_Success(t *testing.T) {
	pubsub := newTestPubSubProvider(t)
	ctx := context.Background()

	streamName := uuid.New().String()
	consumerName := "test-consumer"
	subjects := []string{"test.consumer.>"}

	// Create stream first
	err := pubsub.CreateOrUpdateStream(ctx, streamName, subjects)
	require.NoError(t, err)

	// Create consumer
	err = pubsub.CreateOrUpdateConsumer(ctx, streamName, consumerName, subjects)
	
	assert.NoError(t, err)
}

func TestCreateOrUpdateConsumer_UpdateExisting(t *testing.T) {
	pubsub := newTestPubSubProvider(t)
	ctx := context.Background()

	streamName := uuid.New().String()
	consumerName := "test-consumer"
	subjects := []string{"test.consumer.>"}

	// Create stream
	err := pubsub.CreateOrUpdateStream(ctx, streamName, subjects)
	require.NoError(t, err)

	// Create consumer
	err = pubsub.CreateOrUpdateConsumer(ctx, streamName, consumerName, subjects)
	require.NoError(t, err)

	// Update consumer
	err = pubsub.CreateOrUpdateConsumer(ctx, streamName, consumerName, subjects)
	
	assert.NoError(t, err)
}

func TestSubscribe_Success(t *testing.T) {
	pubsub := newTestPubSubProvider(t)
	ctx := context.Background()

	streamName := uuid.New().String()
	consumerName := "test-consumer"
	subject := "test.subscribe"

	// Create stream
	err := pubsub.CreateOrUpdateStream(ctx, streamName, []string{subject + ".>"})
	require.NoError(t, err)

	// Create consumer
	err = pubsub.CreateOrUpdateConsumer(ctx, streamName, consumerName, []string{subject + ".>"})
	require.NoError(t, err)

	// Subscribe
	messageReceived := make(chan []byte, 1)
	callback := func(msg []byte) bool {
		messageReceived <- msg
		return true
	}

	err = pubsub.Subscribe(ctx, streamName, consumerName, callback)
	assert.NoError(t, err)

	// Publish a message
	data := []byte("test message")
	err = pubsub.Publish(ctx, subject+".1", data)
	require.NoError(t, err)

	// Wait for message
	select {
	case received := <-messageReceived:
		assert.Equal(t, data, received)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for message")
	}
}

func TestPublishSubscribe_MultipleMessages(t *testing.T) {
	pubsub := newTestPubSubProvider(t)
	ctx := context.Background()

	streamName := uuid.New().String()
	consumerName := "test-consumer"
	subject := "test.multiple"

	// Setup stream and consumer
	err := pubsub.CreateOrUpdateStream(ctx, streamName, []string{subject + ".>"})
	require.NoError(t, err)

	err = pubsub.CreateOrUpdateConsumer(ctx, streamName, consumerName, []string{subject + ".>"})
	require.NoError(t, err)

	// Subscribe
	var receivedMessages [][]byte
	var mu sync.Mutex
	callback := func(msg []byte) bool {
		mu.Lock()
		receivedMessages = append(receivedMessages, msg)
		mu.Unlock()
		return true
	}

	err = pubsub.Subscribe(ctx, streamName, consumerName, callback)
	require.NoError(t, err)

	// Publish multiple messages
	numMessages := 10
	for i := 0; i < numMessages; i++ {
		data := []byte(fmt.Sprintf("message-%d", i))
		err = pubsub.Publish(ctx, subject+".1", data)
		require.NoError(t, err)
	}

	// Wait for all messages
	time.Sleep(500 * time.Millisecond)

	mu.Lock()
	assert.Equal(t, numMessages, len(receivedMessages))
	mu.Unlock()
}

func TestSubscribe_CallbackReturnsFalse(t *testing.T) {
	pubsub := newTestPubSubProvider(t)
	ctx := context.Background()

	streamName := uuid.New().String()
	consumerName := "test-consumer"
	subject := "test.nak"

	// Setup stream and consumer
	err := pubsub.CreateOrUpdateStream(ctx, streamName, []string{subject + ".>"})
	require.NoError(t, err)

	err = pubsub.CreateOrUpdateConsumer(ctx, streamName, consumerName, []string{subject + ".>"})
	require.NoError(t, err)

	// Subscribe with callback that returns false (NAK)
	messageCount := 0
	callback := func(msg []byte) bool {
		messageCount++
		return false // NAK the message
	}

	err = pubsub.Subscribe(ctx, streamName, consumerName, callback)
	require.NoError(t, err)

	// Publish a message
	data := []byte("test message")
	err = pubsub.Publish(ctx, subject+".1", data)
	require.NoError(t, err)

	// Wait for message processing
	time.Sleep(200 * time.Millisecond)

	// Should receive message at least once (might get redelivered due to NAK)
	assert.GreaterOrEqual(t, messageCount, 1)
}

func TestUnSubscribe_Success(t *testing.T) {
	pubsub := newTestPubSubProvider(t)
	ctx := context.Background()

	streamName := uuid.New().String()
	consumerName := "test-consumer"
	subject := "test.unsub"

	// Setup and subscribe
	err := pubsub.CreateOrUpdateStream(ctx, streamName, []string{subject + ".>"})
	require.NoError(t, err)

	err = pubsub.CreateOrUpdateConsumer(ctx, streamName, consumerName, []string{subject + ".>"})
	require.NoError(t, err)

	err = pubsub.Subscribe(ctx, streamName, consumerName, func(msg []byte) bool {
		return true
	})
	require.NoError(t, err)

	// Unsubscribe
	err = pubsub.UnSubscribe(consumerName)
	
	assert.NoError(t, err)
}

func TestUnSubscribe_NotSubscribed(t *testing.T) {
	pubsub := newTestPubSubProvider(t)

	// Unsubscribe from non-existent subscription
	err := pubsub.UnSubscribe("nonexistent-consumer")
	
	// Should not error (it's idempotent)
	assert.NoError(t, err)
}

func TestUnSubscribe_StopsReceivingMessages(t *testing.T) {
	pubsub := newTestPubSubProvider(t)
	ctx := context.Background()

	streamName := uuid.New().String()
	consumerName := "test-consumer"
	subject := "test.stoprecv"

	// Setup
	err := pubsub.CreateOrUpdateStream(ctx, streamName, []string{subject + ".>"})
	require.NoError(t, err)

	err = pubsub.CreateOrUpdateConsumer(ctx, streamName, consumerName, []string{subject + ".>"})
	require.NoError(t, err)

	messageCount := 0
	var mu sync.Mutex
	callback := func(msg []byte) bool {
		mu.Lock()
		messageCount++
		mu.Unlock()
		return true
	}

	err = pubsub.Subscribe(ctx, streamName, consumerName, callback)
	require.NoError(t, err)

	// Publish first message
	err = pubsub.Publish(ctx, subject+".1", []byte("message 1"))
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	// Unsubscribe
	err = pubsub.UnSubscribe(consumerName)
	require.NoError(t, err)

	mu.Lock()
	countBeforeUnsub := messageCount
	mu.Unlock()

	// Publish more messages after unsubscribe
	for i := 0; i < 5; i++ {
		err = pubsub.Publish(ctx, subject+".1", []byte(fmt.Sprintf("message %d", i+2)))
		require.NoError(t, err)
	}

	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	countAfterUnsub := messageCount
	mu.Unlock()

	// Should not receive new messages after unsubscribe
	assert.Equal(t, countBeforeUnsub, countAfterUnsub)
}

func TestClose_Success(t *testing.T) {
	s := runEmbeddedNATSServer(t)
	url := fmt.Sprintf("nats://%s", s.Addr().String())

	pubsub, err := NewNatsPubSub(url)
	require.NoError(t, err)

	err = pubsub.Close()
	
	assert.NoError(t, err)
}

func TestClose_Graceful(t *testing.T) {
	s := runEmbeddedNATSServer(t)
	url := fmt.Sprintf("nats://%s", s.Addr().String())

	pubsub, err := NewNatsPubSub(url)
	require.NoError(t, err)

	done := make(chan error)
	go func() {
		done <- pubsub.Close()
	}()

	select {
	case err := <-done:
		assert.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("Close did not complete in time")
	}
}

func TestConcurrentPublish(t *testing.T) {
	pubsub := newTestPubSubProvider(t)
	ctx := context.Background()

	streamName := uuid.New().String()
	subject := "test.concurrent"

	err := pubsub.CreateOrUpdateStream(ctx, streamName, []string{subject + ".>"})
	require.NoError(t, err)

	// Concurrent publishers
	numPublishers := 10
	messagesPerPublisher := 10
	var wg sync.WaitGroup
	wg.Add(numPublishers)

	for i := 0; i < numPublishers; i++ {
		go func(publisherID int) {
			defer wg.Done()
			for j := 0; j < messagesPerPublisher; j++ {
				data := []byte(fmt.Sprintf("publisher-%d-msg-%d", publisherID, j))
				err := pubsub.Publish(ctx, subject+".1", data)
				assert.NoError(t, err)
			}
		}(i)
	}

	wg.Wait()
}

func TestConcurrentSubscribe(t *testing.T) {
	pubsub := newTestPubSubProvider(t)
	ctx := context.Background()

	streamName := uuid.New().String()
	subject := "test.concsub"

	err := pubsub.CreateOrUpdateStream(ctx, streamName, []string{subject + ".>"})
	require.NoError(t, err)

	// Create multiple consumers
	numConsumers := 5
	var wg sync.WaitGroup
	wg.Add(numConsumers)

	for i := 0; i < numConsumers; i++ {
		go func(consumerID int) {
			defer wg.Done()
			consumerName := fmt.Sprintf("consumer-%d", consumerID)
			
			err := pubsub.CreateOrUpdateConsumer(ctx, streamName, consumerName, []string{subject + ".>"})
			assert.NoError(t, err)
			
			err = pubsub.Subscribe(ctx, streamName, consumerName, func(msg []byte) bool {
				return true
			})
			assert.NoError(t, err)
		}(i)
	}

	wg.Wait()
}

func TestReSubscribe_ReplacesOldSubscription(t *testing.T) {
	pubsub := newTestPubSubProvider(t)
	ctx := context.Background()

	streamName := uuid.New().String()
	consumerName := "test-consumer"
	subject := "test.resub"

	// Setup
	err := pubsub.CreateOrUpdateStream(ctx, streamName, []string{subject + ".>"})
	require.NoError(t, err)

	err = pubsub.CreateOrUpdateConsumer(ctx, streamName, consumerName, []string{subject + ".>"})
	require.NoError(t, err)

	// First subscription
	firstCount := 0
	var mu1 sync.Mutex
	err = pubsub.Subscribe(ctx, streamName, consumerName, func(msg []byte) bool {
		mu1.Lock()
		firstCount++
		mu1.Unlock()
		return true
	})
	require.NoError(t, err)

	// Second subscription (should replace first)
	secondCount := 0
	var mu2 sync.Mutex
	err = pubsub.Subscribe(ctx, streamName, consumerName, func(msg []byte) bool {
		mu2.Lock()
		secondCount++
		mu2.Unlock()
		return true
	})
	require.NoError(t, err)

	// Publish messages
	time.Sleep(100 * time.Millisecond) // Let subscriptions settle
	for i := 0; i < 3; i++ {
		err = pubsub.Publish(ctx, subject+".1", []byte(fmt.Sprintf("msg-%d", i)))
		require.NoError(t, err)
	}

	time.Sleep(200 * time.Millisecond)

	// Only second subscription should receive messages
	mu2.Lock()
	assert.Equal(t, 3, secondCount)
	mu2.Unlock()

	mu1.Lock()
	// First callback should not be called anymore
	assert.Equal(t, 0, firstCount)
	mu1.Unlock()
}
