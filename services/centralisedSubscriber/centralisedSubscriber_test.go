package centralisedSubscriber

import (
	"context"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/kychandar/ottam/common"
	"github.com/kychandar/ottam/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mock implementations

type MockPubSubProvider struct {
	mock.Mock
}

func (m *MockPubSubProvider) CreateOrUpdateStream(ctx context.Context, streamName string, subjects []string) error {
	args := m.Called(ctx, streamName, subjects)
	return args.Error(0)
}

func (m *MockPubSubProvider) Publish(ctx context.Context, subjectName string, data []byte) error {
	args := m.Called(ctx, subjectName, data)
	return args.Error(0)
}

func (m *MockPubSubProvider) CreateOrUpdateConsumer(ctx context.Context, streamName, consumerName string, subjects []string) error {
	args := m.Called(ctx, streamName, consumerName, subjects)
	return args.Error(0)
}

func (m *MockPubSubProvider) Subscribe(ctx context.Context, streamName, consumerName string, callBack func(msg []byte) bool) error {
	args := m.Called(ctx, streamName, consumerName, callBack)
	return args.Error(0)
}

func (m *MockPubSubProvider) UnSubscribe(consumerName string) error {
	args := m.Called(consumerName)
	return args.Error(0)
}

func (m *MockPubSubProvider) Close() error {
	args := m.Called()
	return args.Error(0)
}

type MockWsWriteChanManager struct {
	mock.Mock
}

func (m *MockWsWriteChanManager) GetConnectionForClientID(clientID string) (*websocket.Conn, bool) {
	args := m.Called(clientID)
	if conn := args.Get(0); conn != nil {
		return conn.(*websocket.Conn), args.Bool(1)
	}
	return nil, args.Bool(1)
}

func (m *MockWsWriteChanManager) SetConnectionForClientID(clientID string, conn *websocket.Conn) {
	m.Called(clientID, conn)
}

func (m *MockWsWriteChanManager) DeleteClientID(clientID string) {
	m.Called(clientID)
}

func (m *MockWsWriteChanManager) WritePreparedMessage(clientID string, pm *websocket.PreparedMessage) error {
	args := m.Called(clientID, pm)
	return args.Error(0)
}

type MockWorkerPool struct {
	mock.Mock
}

func (m *MockWorkerPool) SubmitMessageJob(ctx context.Context, conn *websocket.Conn, preparedMsg *websocket.PreparedMessage, publishedTime time.Time, msgID string, wsConnID string) {
	m.Called(ctx, conn, preparedMsg, publishedTime, msgID, wsConnID)
}

type MockSerializableMessage struct {
	mock.Mock
}

func (m *MockSerializableMessage) Serialize() ([]byte, error) {
	args := m.Called()
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockSerializableMessage) DeserializeFrom(data []byte) error {
	args := m.Called(data)
	return args.Error(0)
}

func (m *MockSerializableMessage) GetChannelName() common.ChannelName {
	args := m.Called()
	return args.Get(0).(common.ChannelName)
}

func (m *MockSerializableMessage) GetPublishedTime() time.Time {
	args := m.Called()
	return args.Get(0).(time.Time)
}

func (m *MockSerializableMessage) GetMsgID() string {
	args := m.Called()
	return args.String(0)
}

// Test helper to create a subscriber with mocks
func setupSubscriber() (*centralisedSubscriber, *MockPubSubProvider, *MockWsWriteChanManager, *MockWorkerPool, *MockSerializableMessage) {
	mockPubSub := new(MockPubSubProvider)
	mockWsManager := new(MockWsWriteChanManager)
	mockWorkerPool := new(MockWorkerPool)
	mockMsg := new(MockSerializableMessage)

	subscriber := New(
		common.NodeID("test-node"),
		mockWsManager,
		mockWorkerPool,
		func() services.SerializableMessage { return mockMsg },
		mockPubSub,
	).(*centralisedSubscriber)

	return subscriber, mockPubSub, mockWsManager, mockWorkerPool, mockMsg
}

// Tests for Subscribe

func TestSubscribe_NewChannel(t *testing.T) {
	subscriber, _, _, _, _ := setupSubscriber()
	ctx := context.Background()

	clientID := "client-1"
	channelName := common.ChannelName("test-channel")

	err := subscriber.Subscribe(ctx, clientID, channelName)

	assert.NoError(t, err)
	
	// Verify the channel was created and client added
	subMap, exists := subscriber.channelsTracker.Get(channelName)
	assert.True(t, exists, "channel should exist in tracker")
	assert.NotNil(t, subMap)
	
	_, clientExists := subMap.Get(clientID)
	assert.True(t, clientExists, "client should be subscribed to channel")
}

func TestSubscribe_ExistingChannel(t *testing.T) {
	subscriber, _, _, _, _ := setupSubscriber()
	ctx := context.Background()

	channelName := common.ChannelName("test-channel")
	client1 := "client-1"
	client2 := "client-2"

	// First subscription creates the channel
	err := subscriber.Subscribe(ctx, client1, channelName)
	assert.NoError(t, err)

	// Second subscription to same channel
	err = subscriber.Subscribe(ctx, client2, channelName)
	assert.NoError(t, err)

	// Verify both clients are subscribed
	subMap, exists := subscriber.channelsTracker.Get(channelName)
	assert.True(t, exists)
	
	_, client1Exists := subMap.Get(client1)
	assert.True(t, client1Exists)
	
	_, client2Exists := subMap.Get(client2)
	assert.True(t, client2Exists)
}

func TestSubscribe_MultipleChannelsSameClient(t *testing.T) {
	subscriber, _, _, _, _ := setupSubscriber()
	ctx := context.Background()

	clientID := "client-1"
	channel1 := common.ChannelName("channel-1")
	channel2 := common.ChannelName("channel-2")

	err := subscriber.Subscribe(ctx, clientID, channel1)
	assert.NoError(t, err)

	err = subscriber.Subscribe(ctx, clientID, channel2)
	assert.NoError(t, err)

	// Verify client is in both channels
	subMap1, exists1 := subscriber.channelsTracker.Get(channel1)
	assert.True(t, exists1)
	_, inChannel1 := subMap1.Get(clientID)
	assert.True(t, inChannel1)

	subMap2, exists2 := subscriber.channelsTracker.Get(channel2)
	assert.True(t, exists2)
	_, inChannel2 := subMap2.Get(clientID)
	assert.True(t, inChannel2)
}

// Tests for UnSubscribe

func TestUnSubscribe_ClientExists(t *testing.T) {
	subscriber, _, _, _, _ := setupSubscriber()
	ctx := context.Background()

	clientID := "client-1"
	channelName := common.ChannelName("test-channel")

	// First subscribe
	err := subscriber.Subscribe(ctx, clientID, channelName)
	assert.NoError(t, err)

	// Then unsubscribe
	err = subscriber.UnSubscribe(ctx, clientID, channelName)
	assert.NoError(t, err)

	// Verify channel was removed (it was the last client)
	_, exists := subscriber.channelsTracker.Get(channelName)
	assert.False(t, exists, "channel should be removed when last client unsubscribes")
}

func TestUnSubscribe_MultipleClients(t *testing.T) {
	subscriber, _, _, _, _ := setupSubscriber()
	ctx := context.Background()

	channelName := common.ChannelName("test-channel")
	client1 := "client-1"
	client2 := "client-2"

	// Subscribe both clients
	err := subscriber.Subscribe(ctx, client1, channelName)
	assert.NoError(t, err)
	err = subscriber.Subscribe(ctx, client2, channelName)
	assert.NoError(t, err)

	// Unsubscribe one client
	err = subscriber.UnSubscribe(ctx, client1, channelName)
	assert.NoError(t, err)

	// Verify channel still exists
	subMap, exists := subscriber.channelsTracker.Get(channelName)
	assert.True(t, exists, "channel should still exist with remaining client")
	
	// Verify client1 is removed but client2 remains
	_, client1Exists := subMap.Get(client1)
	assert.False(t, client1Exists)
	
	_, client2Exists := subMap.Get(client2)
	assert.True(t, client2Exists)
}

func TestUnSubscribe_NonExistentChannel(t *testing.T) {
	subscriber, _, _, _, _ := setupSubscriber()
	ctx := context.Background()

	clientID := "client-1"
	channelName := common.ChannelName("non-existent-channel")

	// Should not error when unsubscribing from non-existent channel
	err := subscriber.UnSubscribe(ctx, clientID, channelName)
	assert.NoError(t, err)
}

func TestUnSubscribe_NonExistentClient(t *testing.T) {
	subscriber, _, _, _, _ := setupSubscriber()
	ctx := context.Background()

	channelName := common.ChannelName("test-channel")
	client1 := "client-1"
	client2 := "client-2"

	// Subscribe client1
	err := subscriber.Subscribe(ctx, client1, channelName)
	assert.NoError(t, err)

	// Try to unsubscribe client2 (doesn't exist)
	err = subscriber.UnSubscribe(ctx, client2, channelName)
	assert.NoError(t, err)

	// Verify client1 is still subscribed
	subMap, exists := subscriber.channelsTracker.Get(channelName)
	assert.True(t, exists)
	_, client1Exists := subMap.Get(client1)
	assert.True(t, client1Exists)
}

// Tests for UnsubscribeAll

func TestUnsubscribeAll_SingleChannel(t *testing.T) {
	subscriber, _, _, _, _ := setupSubscriber()
	ctx := context.Background()

	clientID := "client-1"
	channelName := common.ChannelName("test-channel")

	// Subscribe to a channel
	err := subscriber.Subscribe(ctx, clientID, channelName)
	assert.NoError(t, err)

	// Unsubscribe from all
	err = subscriber.UnsubscribeAll(ctx, clientID)
	assert.NoError(t, err)

	// Verify channel is removed
	_, exists := subscriber.channelsTracker.Get(channelName)
	assert.False(t, exists)
}

func TestUnsubscribeAll_MultipleChannels(t *testing.T) {
	subscriber, _, _, _, _ := setupSubscriber()
	ctx := context.Background()

	clientID := "client-1"
	channel1 := common.ChannelName("channel-1")
	channel2 := common.ChannelName("channel-2")
	channel3 := common.ChannelName("channel-3")

	// Subscribe to multiple channels
	subscriber.Subscribe(ctx, clientID, channel1)
	subscriber.Subscribe(ctx, clientID, channel2)
	subscriber.Subscribe(ctx, clientID, channel3)

	// Unsubscribe from all
	err := subscriber.UnsubscribeAll(ctx, clientID)
	assert.NoError(t, err)

	// Verify all channels are removed (client was the only subscriber)
	_, exists1 := subscriber.channelsTracker.Get(channel1)
	assert.False(t, exists1)
	_, exists2 := subscriber.channelsTracker.Get(channel2)
	assert.False(t, exists2)
	_, exists3 := subscriber.channelsTracker.Get(channel3)
	assert.False(t, exists3)
}

func TestUnsubscribeAll_SharedChannels(t *testing.T) {
	subscriber, _, _, _, _ := setupSubscriber()
	ctx := context.Background()

	client1 := "client-1"
	client2 := "client-2"
	channelName := common.ChannelName("shared-channel")

	// Both clients subscribe to same channel
	subscriber.Subscribe(ctx, client1, channelName)
	subscriber.Subscribe(ctx, client2, channelName)

	// Unsubscribe client1 from all
	err := subscriber.UnsubscribeAll(ctx, client1)
	assert.NoError(t, err)

	// Verify channel still exists (client2 is still subscribed)
	subMap, exists := subscriber.channelsTracker.Get(channelName)
	assert.True(t, exists, "channel should still exist")
	
	// Verify client1 is removed but client2 remains
	_, client1Exists := subMap.Get(client1)
	assert.False(t, client1Exists)
	_, client2Exists := subMap.Get(client2)
	assert.True(t, client2Exists)
}

func TestUnsubscribeAll_NoSubscriptions(t *testing.T) {
	subscriber, _, _, _, _ := setupSubscriber()
	ctx := context.Background()

	clientID := "client-1"

	// Unsubscribe from all when no subscriptions exist
	err := subscriber.UnsubscribeAll(ctx, clientID)
	assert.NoError(t, err)
}

// Tests for SubscriptionSyncer

func TestSubscriptionSyncer_UpdatesConsumer(t *testing.T) {
	subscriber, mockPubSub, _, _, _ := setupSubscriber()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	channelName := common.ChannelName("test-channel")

	// Set up mock expectations
	mockPubSub.On("CreateOrUpdateConsumer", 
		mock.Anything, 
		common.OttamServerStreamName, 
		string(subscriber.nodeID), 
		[]string{common.ChannelSubjFormat(string(channelName))}).Return(nil).Once()

	// Start syncer in background
	go subscriber.SubscriptionSyncer(ctx)

	// Give it time to start
	time.Sleep(10 * time.Millisecond)

	// Subscribe to trigger syncer
	subscriber.Subscribe(ctx, "client-1", channelName)

	// Wait for syncer to process
	time.Sleep(50 * time.Millisecond)

	// Verify mock was called
	mockPubSub.AssertExpectations(t)
}

func TestSubscriptionSyncer_ContextCancellation(t *testing.T) {
	subscriber, _, _, _, _ := setupSubscriber()
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan bool)
	go func() {
		subscriber.SubscriptionSyncer(ctx)
		done <- true
	}()

	// Cancel context
	cancel()

	// Wait for syncer to stop
	select {
	case <-done:
		// Success
	case <-time.After(100 * time.Millisecond):
		t.Fatal("SubscriptionSyncer did not stop after context cancellation")
	}
}

// Tests for New constructor

func TestNew_CreatesValidInstance(t *testing.T) {
	mockPubSub := new(MockPubSubProvider)
	mockWsManager := new(MockWsWriteChanManager)
	mockWorkerPool := new(MockWorkerPool)
	
	nodeID := common.NodeID("test-node")
	newMsgGetter := func() services.SerializableMessage { return new(MockSerializableMessage) }

	subscriber := New(nodeID, mockWsManager, mockWorkerPool, newMsgGetter, mockPubSub).(*centralisedSubscriber)

	assert.NotNil(t, subscriber)
	assert.Equal(t, nodeID, subscriber.nodeID)
	assert.NotNil(t, subscriber.channelsTracker)
	assert.NotNil(t, subscriber.fanoutCh)
	assert.NotNil(t, subscriber.stopFanout)
	assert.NotNil(t, subscriber.subscriptionSyncer)
	assert.Equal(t, mockPubSub, subscriber.pubSubProvider)
	assert.Equal(t, mockWsManager, subscriber.wsWriteChanManager)
	assert.Equal(t, mockWorkerPool, subscriber.workerPool)
}

// Concurrency tests

func TestSubscribe_ConcurrentAccess(t *testing.T) {
	subscriber, _, _, _, _ := setupSubscriber()
	ctx := context.Background()

	channelName := common.ChannelName("test-channel")
	numClients := 100

	done := make(chan bool, numClients)

	// Concurrent subscriptions
	for i := 0; i < numClients; i++ {
		go func(clientNum int) {
			clientID := string(rune('A' + clientNum%26)) + string(rune('0' + clientNum/26))
			err := subscriber.Subscribe(ctx, clientID, channelName)
			assert.NoError(t, err)
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < numClients; i++ {
		<-done
	}

	// Verify all clients are subscribed
	subMap, exists := subscriber.channelsTracker.Get(channelName)
	assert.True(t, exists)
	assert.Equal(t, uintptr(numClients), subMap.Len())
}

func TestUnSubscribe_ConcurrentAccess(t *testing.T) {
	subscriber, _, _, _, _ := setupSubscriber()
	ctx := context.Background()

	channelName := common.ChannelName("test-channel")
	numClients := 50

	// First, subscribe all clients
	clientIDs := make([]string, numClients)
	for i := 0; i < numClients; i++ {
		clientIDs[i] = string(rune('A' + i%26)) + string(rune('0' + i/26))
		err := subscriber.Subscribe(ctx, clientIDs[i], channelName)
		assert.NoError(t, err)
	}

	done := make(chan bool, numClients)

	// Concurrent unsubscriptions
	for i := 0; i < numClients; i++ {
		go func(clientID string) {
			err := subscriber.UnSubscribe(ctx, clientID, channelName)
			assert.NoError(t, err)
			done <- true
		}(clientIDs[i])
	}

	// Wait for all goroutines
	for i := 0; i < numClients; i++ {
		<-done
	}

	// Verify channel is removed (all clients unsubscribed)
	_, exists := subscriber.channelsTracker.Get(channelName)
	assert.False(t, exists)
}