package http

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/kychandar/ottam/common"
	"github.com/kychandar/ottam/config"
	"github.com/kychandar/ottam/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Mock implementations

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

type MockWebSocketBridge struct {
	mock.Mock
}

func (m *MockWebSocketBridge) ProcessMessagesFromServer(ctx context.Context) {
	m.Called(ctx)
}

func (m *MockWebSocketBridge) ProcessMessagesFromClient(ctx context.Context) {
	m.Called(ctx)
}

type MockMetricsRegistry struct {
	mock.Mock
}

func (m *MockMetricsRegistry) GetHandler() http.Handler {
	args := m.Called()
	if handler := args.Get(0); handler != nil {
		return handler.(http.Handler)
	}
	return nil
}

func (m *MockMetricsRegistry) IncWsConnectionCount() {
	m.Called()
}

func (m *MockMetricsRegistry) DecWsConnectionCount() {
	m.Called()
}

func (m *MockMetricsRegistry) ObserveCentSubscriberStartLat(msgPublishedTime time.Time) {
	m.Called(msgPublishedTime)
}

func (m *MockMetricsRegistry) ObserveCentSubscriberFinishLat(msgPublishedTime time.Time) {
	m.Called(msgPublishedTime)
}

func (m *MockMetricsRegistry) ObserveWsBridgeWriteStart(msgPublishedTime time.Time) {
	m.Called(msgPublishedTime)
}

func (m *MockMetricsRegistry) ObserveWsBridgeWriteEnd(msgPublishedTime time.Time) {
	m.Called(msgPublishedTime)
}

// Test helper functions

func createTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
}

func createTestConfig() *config.Config {
	cfg := &config.Config{}
	cfg.Server.Host = "0.0.0.0"
	cfg.Server.Port = 8080
	cfg.Server.ShutdownTimeout = 30
	cfg.Server.TLS.Enabled = false
	cfg.PubSub.Provider = "nats"
	cfg.PubSub.URL = "nats://localhost:4222"
	cfg.PubSub.TLS.Enabled = false
	cfg.Health.Enabled = true
	cfg.Health.Port = 8081
	cfg.Health.ReadinessPath = "/health/ready"
	cfg.Health.LivenessPath = "/health/live"
	return cfg
}

func TestNewWorkerPool(t *testing.T) {
	logger := createTestLogger()
	mockWsManager := new(MockWsWriteChanManager)
	mockMetrics := new(MockMetricsRegistry)

	pool := NewWorkerPool(logger, mockWsManager, mockMetrics)

	assert.NotNil(t, pool)

	// Verify it implements the interface
	var _ services.WorkerPool = pool

	// Cast to workerPool to check internal state
	wp := pool.(*workerPool)
	assert.NotNil(t, wp.jobQueue)
	assert.Equal(t, logger, wp.logger)
	assert.Equal(t, mockWsManager, wp.wsWriteChanManager)
	assert.Equal(t, mockMetrics, wp.metricsRegistry)
}

func TestWorkerPool_SubmitMessageJob(t *testing.T) {
	logger := createTestLogger()
	mockWsManager := new(MockWsWriteChanManager)
	mockMetrics := new(MockMetricsRegistry)

	// Set up mock expectation since workers will process the job
	mockWsManager.On("WritePreparedMessage", mock.Anything, mock.Anything).Return(nil)
	mockMetrics.On("ObserveWsBridgeWriteStart", mock.Anything).Return()
	mockMetrics.On("ObserveWsBridgeWriteEnd", mock.Anything).Return()

	pool := NewWorkerPool(logger, mockWsManager, mockMetrics)
	wp := pool.(*workerPool)

	ctx := context.Background()
	pm, err := websocket.NewPreparedMessage(websocket.TextMessage, []byte("test"))
	require.NoError(t, err)

	publishedTime := time.Now()
	msgID := "msg-123"
	wsConnID := "conn-456"

	// Submit a job
	pool.SubmitMessageJob(ctx, nil, pm, publishedTime, msgID, wsConnID)

	// Give it a moment to be processed
	time.Sleep(50 * time.Millisecond)

	// Verify job queue exists
	assert.NotNil(t, wp.jobQueue)

	// Verify the job was processed
	mockWsManager.AssertCalled(t, "WritePreparedMessage", wsConnID, pm)
}

func TestWorkerPool_SubmitMessageJob_Success(t *testing.T) {
	logger := createTestLogger()
	mockWsManager := new(MockWsWriteChanManager)
	mockMetrics := new(MockMetricsRegistry)

	mockMetrics.On("ObserveWsBridgeWriteStart", mock.Anything).Return()
	mockMetrics.On("ObserveWsBridgeWriteEnd", mock.Anything).Return()

	pool := NewWorkerPool(logger, mockWsManager, mockMetrics)

	ctx := context.Background()
	pm, err := websocket.NewPreparedMessage(websocket.TextMessage, []byte("test message"))
	require.NoError(t, err)

	publishedTime := time.Now()
	msgID := "msg-789"
	wsConnID := "conn-789"

	// Set up mock to succeed
	mockWsManager.On("WritePreparedMessage", wsConnID, pm).Return(nil)

	// Submit job
	pool.SubmitMessageJob(ctx, nil, pm, publishedTime, msgID, wsConnID)

	// Wait for processing
	time.Sleep(50 * time.Millisecond)

	// Verify mock was called
	mockWsManager.AssertExpectations(t)
}

func TestWorkerPool_SubmitMessageJob_Error(t *testing.T) {
	logger := createTestLogger()
	mockWsManager := new(MockWsWriteChanManager)
	mockMetrics := new(MockMetricsRegistry)

	mockMetrics.On("ObserveWsBridgeWriteStart", mock.Anything).Return()

	pool := NewWorkerPool(logger, mockWsManager, mockMetrics)

	ctx := context.Background()
	pm, err := websocket.NewPreparedMessage(websocket.TextMessage, []byte("test"))
	require.NoError(t, err)

	publishedTime := time.Now()
	msgID := "msg-error"
	wsConnID := "conn-error"

	// Set up mock to return error
	mockWsManager.On("WritePreparedMessage", wsConnID, pm).Return(websocket.ErrCloseSent)

	// Submit job
	pool.SubmitMessageJob(ctx, nil, pm, publishedTime, msgID, wsConnID)

	// Wait for processing
	time.Sleep(50 * time.Millisecond)

	// Verify mock was called even with error
	mockWsManager.AssertExpectations(t)
}

func TestWorkerPool_SubmitMessageJob_ContextCancelled(t *testing.T) {
	logger := createTestLogger()
	mockWsManager := new(MockWsWriteChanManager)
	mockMetrics := new(MockMetricsRegistry)

	pool := NewWorkerPool(logger, mockWsManager, mockMetrics)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	pm, err := websocket.NewPreparedMessage(websocket.TextMessage, []byte("test"))
	require.NoError(t, err)

	publishedTime := time.Now()
	msgID := "msg-cancelled"
	wsConnID := "conn-cancelled"

	// Submit job with cancelled context
	pool.SubmitMessageJob(ctx, nil, pm, publishedTime, msgID, wsConnID)

	// Wait a bit
	time.Sleep(50 * time.Millisecond)

	// With cancelled context, the worker should skip the message
	// So WritePreparedMessage should NOT be called
}

func TestWorkerPool_MultipleJobs(t *testing.T) {
	logger := createTestLogger()
	mockWsManager := new(MockWsWriteChanManager)
	mockMetrics := new(MockMetricsRegistry)

	mockMetrics.On("ObserveWsBridgeWriteStart", mock.Anything).Return()
	mockMetrics.On("ObserveWsBridgeWriteEnd", mock.Anything).Return()

	pool := NewWorkerPool(logger, mockWsManager, mockMetrics)

	ctx := context.Background()
	numJobs := 100

	// Set up mock to handle all jobs
	mockWsManager.On("WritePreparedMessage", mock.Anything, mock.Anything).Return(nil).Times(numJobs)

	// Submit multiple jobs
	for i := 0; i < numJobs; i++ {
		pm, _ := websocket.NewPreparedMessage(websocket.TextMessage, []byte("test"))
		pool.SubmitMessageJob(ctx, nil, pm, time.Now(), "msg", "conn")
	}

	// Wait for all jobs to process
	time.Sleep(200 * time.Millisecond)

	// Verify all were processed
	mockWsManager.AssertExpectations(t)
}

func TestNew(t *testing.T) {
	logger := createTestLogger()
	mockCentSub := new(MockCentralisedSubscriber)
	mockWsManager := new(MockWsWriteChanManager)
	mockMetrics := new(MockMetricsRegistry)
	cfg := createTestConfig()

	bridgeFactory := func(centSubscriber services.CentralisedSubscriber, wsConnID string, conn *websocket.Conn) services.WebSocketBridge {
		return new(MockWebSocketBridge)
	}

	srv := New(mockCentSub, bridgeFactory, mockWsManager, mockMetrics, logger, cfg)

	assert.NotNil(t, srv)
	assert.Equal(t, mockCentSub, srv.centSubscriber)
	assert.Equal(t, mockWsManager, srv.wsWriteChanManager)
	assert.Equal(t, mockMetrics, srv.metricsRegistry)
	assert.Equal(t, logger, srv.logger)
	assert.Equal(t, cfg, srv.config)
	assert.NotNil(t, srv.workerPool)
	assert.NotNil(t, srv.wsBridgeFactory)
	assert.NotNil(t, srv.healthChecker)
}

func TestServer_ServeHTTP(t *testing.T) {
	logger := createTestLogger()
	mockCentSub := new(MockCentralisedSubscriber)
	mockWsManager := new(MockWsWriteChanManager)
	mockMetrics := new(MockMetricsRegistry)
	mockBridge := new(MockWebSocketBridge)
	cfg := createTestConfig()

	bridgeFactory := func(centSubscriber services.CentralisedSubscriber, wsConnID string, conn *websocket.Conn) services.WebSocketBridge {
		return mockBridge
	}

	srv := New(mockCentSub, bridgeFactory, mockWsManager, mockMetrics, logger, cfg)

	// Set up mock expectations
	mockWsManager.On("SetConnectionForClientID", mock.Anything, mock.Anything).Return()
	mockWsManager.On("DeleteClientID", mock.Anything).Return()
	mockCentSub.On("UnsubscribeAll", mock.Anything, mock.Anything).Return(nil)
	mockBridge.On("ProcessMessagesFromClient", mock.Anything).Return()
	mockMetrics.On("IncWsConnectionCount").Return()
	mockMetrics.On("DecWsConnectionCount").Return()

	// Create test server
	testServer := httptest.NewServer(http.HandlerFunc(srv.ServeHTTP))
	defer testServer.Close()

	// Connect via WebSocket
	wsURL := "ws" + strings.TrimPrefix(testServer.URL, "http")
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer ws.Close()

	// Give it time to process
	time.Sleep(100 * time.Millisecond)

	// Close connection
	ws.Close()

	// Wait for cleanup
	time.Sleep(100 * time.Millisecond)

	// Verify mocks were called
	mockWsManager.AssertCalled(t, "SetConnectionForClientID", mock.Anything, mock.Anything)
	mockWsManager.AssertCalled(t, "DeleteClientID", mock.Anything)
	mockCentSub.AssertCalled(t, "UnsubscribeAll", mock.Anything, mock.Anything)
	mockBridge.AssertCalled(t, "ProcessMessagesFromClient", mock.Anything)
}

func TestServer_ServeHTTP_MultipleConnections(t *testing.T) {
	logger := createTestLogger()
	mockCentSub := new(MockCentralisedSubscriber)
	mockWsManager := new(MockWsWriteChanManager)
	mockMetrics := new(MockMetricsRegistry)
	cfg := createTestConfig()

	bridgeFactory := func(centSubscriber services.CentralisedSubscriber, wsConnID string, conn *websocket.Conn) services.WebSocketBridge {
		bridge := new(MockWebSocketBridge)
		bridge.On("ProcessMessagesFromClient", mock.Anything).Return()
		return bridge
	}

	srv := New(mockCentSub, bridgeFactory, mockWsManager, mockMetrics, logger, cfg)

	// Set up mock expectations for multiple connections
	mockWsManager.On("SetConnectionForClientID", mock.Anything, mock.Anything).Return()
	mockWsManager.On("DeleteClientID", mock.Anything).Return()
	mockCentSub.On("UnsubscribeAll", mock.Anything, mock.Anything).Return(nil)
	mockMetrics.On("IncWsConnectionCount").Return()
	mockMetrics.On("DecWsConnectionCount").Return()

	// Create test server
	testServer := httptest.NewServer(http.HandlerFunc(srv.ServeHTTP))
	defer testServer.Close()

	// Connect multiple WebSocket clients
	numClients := 10
	connections := make([]*websocket.Conn, numClients)
	wsURL := "ws" + strings.TrimPrefix(testServer.URL, "http")

	for i := 0; i < numClients; i++ {
		ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		require.NoError(t, err)
		connections[i] = ws
	}

	// Give time to process
	time.Sleep(100 * time.Millisecond)

	// Close all connections
	for _, conn := range connections {
		conn.Close()
	}

	// Wait for cleanup
	time.Sleep(100 * time.Millisecond)

	// Verify mocks were called correct number of times
	mockWsManager.AssertNumberOfCalls(t, "SetConnectionForClientID", numClients)
	mockWsManager.AssertNumberOfCalls(t, "DeleteClientID", numClients)
	mockCentSub.AssertNumberOfCalls(t, "UnsubscribeAll", numClients)
}

func TestUpgrader_CheckOrigin(t *testing.T) {
	t.Run("accepts all origins", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/ws", nil)
		req.Header.Set("Origin", "http://example.com")

		result := upgrader.CheckOrigin(req)
		assert.True(t, result)
	})

	t.Run("accepts localhost", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/ws", nil)
		req.Header.Set("Origin", "http://localhost:3000")

		result := upgrader.CheckOrigin(req)
		assert.True(t, result)
	})

	t.Run("accepts any origin", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/ws", nil)
		req.Header.Set("Origin", "https://malicious.com")

		result := upgrader.CheckOrigin(req)
		assert.True(t, result)
	})
}

func TestConstants(t *testing.T) {
	t.Run("maxWorkers is set correctly", func(t *testing.T) {
		assert.Equal(t, 1000, maxWorkers)
	})

	t.Run("jobQueueSize is set correctly", func(t *testing.T) {
		assert.Equal(t, 30000, jobQueueSize)
	})

	t.Run("timeout constants", func(t *testing.T) {
		assert.Equal(t, 10*time.Second, writeWait)
		assert.Equal(t, 60*time.Second, pongWait)
		assert.Equal(t, 54*time.Second, pingPeriod)
		assert.Equal(t, 512*1024, maxMessageSize)
	})
}

func TestMessageJob_Structure(t *testing.T) {
	pm, err := websocket.NewPreparedMessage(websocket.TextMessage, []byte("test"))
	require.NoError(t, err)

	ctx := context.Background()
	publishedTime := time.Now()
	msgID := "msg-123"
	wsConnID := "conn-123"

	job := messageJob{
		preparedMsg:   pm,
		publishedTime: publishedTime,
		msgID:         msgID,
		ctx:           ctx,
		wsConnID:      wsConnID,
	}

	assert.Equal(t, pm, job.preparedMsg)
	assert.Equal(t, publishedTime, job.publishedTime)
	assert.Equal(t, msgID, job.msgID)
	assert.Equal(t, ctx, job.ctx)
	assert.Equal(t, wsConnID, job.wsConnID)
}

func TestWorkerPool_QueueFull(t *testing.T) {
	logger := createTestLogger()
	mockWsManager := new(MockWsWriteChanManager)

	// Create a worker pool
	pool := &workerPool{
		jobQueue:           make(chan messageJob, 2), // Very small queue
		logger:             logger,
		wsWriteChanManager: mockWsManager,
	}

	// Don't start workers so jobs pile up
	ctx := context.Background()
	pm, _ := websocket.NewPreparedMessage(websocket.TextMessage, []byte("test"))

	// Fill the queue
	pool.SubmitMessageJob(ctx, nil, pm, time.Now(), "msg1", "conn1")
	pool.SubmitMessageJob(ctx, nil, pm, time.Now(), "msg2", "conn2")

	// This one should be dropped (queue full)
	pool.SubmitMessageJob(ctx, nil, pm, time.Now(), "msg3", "conn3")

	// Verify queue is full
	assert.Equal(t, 2, len(pool.jobQueue))
}

func TestServer_UpgraderConfiguration(t *testing.T) {
	t.Run("buffer sizes are optimized", func(t *testing.T) {
		assert.Equal(t, 4096, upgrader.ReadBufferSize)
		assert.Equal(t, 4096, upgrader.WriteBufferSize)
	})
}

func TestWorkerPool_ConcurrentSubmit(t *testing.T) {
	logger := createTestLogger()
	mockWsManager := new(MockWsWriteChanManager)
	mockMetrics := new(MockMetricsRegistry)

	mockWsManager.On("WritePreparedMessage", mock.Anything, mock.Anything).Return(nil)
	mockMetrics.On("ObserveWsBridgeWriteStart", mock.Anything).Return()
	mockMetrics.On("ObserveWsBridgeWriteEnd", mock.Anything).Return()

	pool := NewWorkerPool(logger, mockWsManager, mockMetrics)

	ctx := context.Background()
	numGoroutines := 50
	jobsPerGoroutine := 10

	done := make(chan bool, numGoroutines)

	// Submit jobs concurrently from multiple goroutines
	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			for j := 0; j < jobsPerGoroutine; j++ {
				pm, _ := websocket.NewPreparedMessage(websocket.TextMessage, []byte("test"))
				pool.SubmitMessageJob(ctx, nil, pm, time.Now(), "msg", "conn")
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines to finish
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Give workers time to process
	time.Sleep(300 * time.Millisecond)

	// Should not panic or have race conditions
}
