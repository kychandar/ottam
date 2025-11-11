package websocketbridge

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/kychandar/ottam/common"
	"github.com/kychandar/ottam/common/ds"
	"github.com/kychandar/ottam/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Mock CentralisedSubscriber
type MockCentralisedSubscriber struct {
	mock.Mock
}

func (m *MockCentralisedSubscriber) Subscribe(ctx context.Context, clientID string, channelName common.ChannelName) error {
	args := m.Called(ctx, clientID, channelName)
	return args.Error(0)
}

func (m *MockCentralisedSubscriber) UnSubscribe(ctx context.Context, clientID string, channelName common.ChannelName) error {
	args := m.Called(ctx, clientID, channelName)
	return args.Error(0)
}

func (m *MockCentralisedSubscriber) UnsubscribeAll(ctx context.Context, clientID string) error {
	args := m.Called(ctx, clientID)
	return args.Error(0)
}

func (m *MockCentralisedSubscriber) SubscriptionSyncer(ctx context.Context) {
	m.Called(ctx)
}

func (m *MockCentralisedSubscriber) ProcessDownstreamMessages(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func TestNewWsBridgeFactory(t *testing.T) {
	factory := NewWsBridgeFactory()
	assert.NotNil(t, factory)

	mockSub := new(MockCentralisedSubscriber)
	conn, _, cleanup := createTestWebSocketConnection(t)
	defer cleanup()

	bridge := factory(mockSub, "test-client", conn)
	assert.NotNil(t, bridge)

	// Verify it's the correct type
	wsBridge, ok := bridge.(*websocketBridge)
	assert.True(t, ok)
	assert.Equal(t, "test-client", wsBridge.wsConnID)
	assert.Equal(t, conn, wsBridge.conn)
	assert.Equal(t, mockSub, wsBridge.centSubscriber)
}

func TestProcessMessagesFromServer(t *testing.T) {
	mockSub := new(MockCentralisedSubscriber)
	conn, _, cleanup := createTestWebSocketConnection(t)
	defer cleanup()

	bridge := &websocketBridge{
		wsConnID:       "test-client",
		conn:           conn,
		centSubscriber: mockSub,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// This method should just wait for context cancellation
	done := make(chan bool)
	go func() {
		bridge.ProcessMessagesFromServer(ctx)
		done <- true
	}()

	// Wait for context to be done
	select {
	case <-done:
		// Success - method returned when context was cancelled
	case <-time.After(200 * time.Millisecond):
		t.Fatal("ProcessMessagesFromServer did not return after context cancellation")
	}
}

func TestHandleControlOp_StartSubscription(t *testing.T) {
	mockSub := new(MockCentralisedSubscriber)

	bridge := &websocketBridge{
		wsConnID:       "test-client-123",
		conn:           nil,
		centSubscriber: mockSub,
	}

	// Prepare subscription payload
	payload := ds.SubscriptionPayload{
		ChannelName: "test-channel",
	}
	data, err := json.Marshal(payload)
	require.NoError(t, err)

	// Set up mock expectations
	mockSub.On("Subscribe", mock.Anything, "test-client-123", common.ChannelName("test-channel")).Return(nil)

	// Call HandleControlOp
	err = bridge.HandleControlOp(ds.StartSubscription, data)
	assert.NoError(t, err)

	// Verify mock was called
	mockSub.AssertExpectations(t)
}

func TestHandleControlOp_StopSubscription(t *testing.T) {
	mockSub := new(MockCentralisedSubscriber)

	bridge := &websocketBridge{
		wsConnID:       "test-client-456",
		conn:           nil,
		centSubscriber: mockSub,
	}

	// Prepare subscription payload
	payload := ds.SubscriptionPayload{
		ChannelName: "stop-channel",
	}
	data, err := json.Marshal(payload)
	require.NoError(t, err)

	// Set up mock expectations
	mockSub.On("UnSubscribe", mock.Anything, "test-client-456", common.ChannelName("stop-channel")).Return(nil)

	// Call HandleControlOp
	err = bridge.HandleControlOp(ds.StopSubscription, data)
	assert.NoError(t, err)

	// Verify mock was called
	mockSub.AssertExpectations(t)
}

func TestHandleControlOp_UnknownOperation(t *testing.T) {
	mockSub := new(MockCentralisedSubscriber)

	bridge := &websocketBridge{
		wsConnID:       "test-client",
		conn:           nil,
		centSubscriber: mockSub,
	}

	// Try with an unknown operation
	err := bridge.HandleControlOp(ds.ControlPlaneOp(999), []byte("{}"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown control op")
}

func TestHandleControlOp_InvalidJSON(t *testing.T) {
	mockSub := new(MockCentralisedSubscriber)

	bridge := &websocketBridge{
		wsConnID:       "test-client",
		conn:           nil,
		centSubscriber: mockSub,
	}

	// Try with invalid JSON
	err := bridge.HandleControlOp(ds.StartSubscription, []byte("invalid json"))
	assert.Error(t, err)
}

func TestHandleSubscriptionOp_StartSubscription(t *testing.T) {
	mockSub := new(MockCentralisedSubscriber)

	bridge := &websocketBridge{
		wsConnID:       "client-id",
		conn:           nil,
		centSubscriber: mockSub,
	}

	payload := ds.SubscriptionPayload{
		ChannelName: "channel-1",
	}
	data, _ := json.Marshal(payload)

	mockSub.On("Subscribe", mock.Anything, "client-id", common.ChannelName("channel-1")).Return(nil)

	err := bridge.handleSubscriptionOp(ds.StartSubscription, data)
	assert.NoError(t, err)
	mockSub.AssertExpectations(t)
}

func TestHandleSubscriptionOp_StopSubscription(t *testing.T) {
	mockSub := new(MockCentralisedSubscriber)

	bridge := &websocketBridge{
		wsConnID:       "client-id",
		conn:           nil,
		centSubscriber: mockSub,
	}

	payload := ds.SubscriptionPayload{
		ChannelName: "channel-2",
	}
	data, _ := json.Marshal(payload)

	mockSub.On("UnSubscribe", mock.Anything, "client-id", common.ChannelName("channel-2")).Return(nil)

	err := bridge.handleSubscriptionOp(ds.StopSubscription, data)
	assert.NoError(t, err)
	mockSub.AssertExpectations(t)
}

func TestHandleSubscriptionOp_InvalidJSON(t *testing.T) {
	mockSub := new(MockCentralisedSubscriber)

	bridge := &websocketBridge{
		wsConnID:       "client-id",
		conn:           nil,
		centSubscriber: mockSub,
	}

	err := bridge.handleSubscriptionOp(ds.StartSubscription, []byte("not valid json"))
	assert.Error(t, err)
}

func TestProcessMessagesFromClient_ControlPlaneMessage(t *testing.T) {
	mockSub := new(MockCentralisedSubscriber)

	// Create a test server that will send messages
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()

		// Send a control plane message for subscription
		controlMsg := ds.ControlPlaneMessage{
			Version:        1,
			ControlPlaneOp: ds.StartSubscription,
			Payload: ds.SubscriptionPayload{
				ChannelName: "test-channel",
			},
		}

		clientMsg := ds.ClientMessage{
			Version:               1,
			Id:                    "msg-1",
			IsControlPlaneMessage: true,
			Payload:               controlMsg,
		}

		data, _ := json.Marshal(clientMsg)
		conn.WriteMessage(websocket.BinaryMessage, data)

		// Keep connection open briefly
		time.Sleep(100 * time.Millisecond)
	}))
	defer server.Close()

	// Connect to the test server
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	clientConn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer clientConn.Close()

	bridge := &websocketBridge{
		wsConnID:       "test-client",
		conn:           clientConn,
		centSubscriber: mockSub,
	}

	// Set up mock expectation
	mockSub.On("Subscribe", mock.Anything, "test-client", common.ChannelName("test-channel")).Return(nil)

	// Start processing messages in background
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	done := make(chan bool)
	go func() {
		bridge.ProcessMessagesFromClient(ctx)
		done <- true
	}()

	// Wait for processing to complete
	<-done

	// Verify mock was called
	mockSub.AssertExpectations(t)
}

func TestProcessMessagesFromClient_NonControlPlaneMessage(t *testing.T) {
	mockSub := new(MockCentralisedSubscriber)

	// Create a test server that will send messages
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()

		// Send a non-control plane message
		clientMsg := ds.ClientMessage{
			Version:               1,
			Id:                    "msg-2",
			IsControlPlaneMessage: false,
			Payload:               "some data payload",
		}

		data, _ := json.Marshal(clientMsg)
		conn.WriteMessage(websocket.BinaryMessage, data)

		// Keep connection open briefly
		time.Sleep(100 * time.Millisecond)
	}))
	defer server.Close()

	// Connect to the test server
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	clientConn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer clientConn.Close()

	bridge := &websocketBridge{
		wsConnID:       "test-client",
		conn:           clientConn,
		centSubscriber: mockSub,
	}

	// Start processing messages in background
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	done := make(chan bool)
	go func() {
		bridge.ProcessMessagesFromClient(ctx)
		done <- true
	}()

	// Wait for processing to complete
	<-done

	// No mock expectations needed for non-control plane messages
}

// Helper to create a test WebSocket connection
func createTestWebSocketConnection(t *testing.T) (*websocket.Conn, *websocket.Conn, func()) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		// Keep connection alive for tests
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				break
			}
		}
	}))

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	clientConn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)

	time.Sleep(10 * time.Millisecond)

	cleanup := func() {
		clientConn.Close()
		server.Close()
	}

	return clientConn, clientConn, cleanup
}

func TestWebsocketBridge_Integration(t *testing.T) {
	mockSub := new(MockCentralisedSubscriber)
	conn, _, cleanup := createTestWebSocketConnection(t)
	defer cleanup()

	factory := NewWsBridgeFactory()
	bridge := factory(mockSub, "integration-test-client", conn)

	assert.NotNil(t, bridge)

	// Test that it implements the interface
	var _ services.WebSocketBridge = bridge
}
