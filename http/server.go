package http

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/kychandar/ottam/common"
	"github.com/kychandar/ottam/metrics"
	"github.com/kychandar/ottam/services"
	slogctx "github.com/veqryn/slog-context"
)

const (
	// Optimized for 4 CPU / 8GiB / 5k connections per pod
	maxWorkers   = 1000  // 1 worker per 5 connections (5000/5)
	jobQueueSize = 30000 // 6x workers, handles broadcast to all connections

	// WebSocket timeout constants
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 512 * 1024
)

type messageJob struct {
	preparedMsg   *websocket.PreparedMessage
	publishedTime time.Time
	msgID         string
	ctx           context.Context
	wsConnID      string
}

type workerPool struct {
	jobQueue           chan messageJob
	logger             *slog.Logger
	wsWriteChanManager services.WsWriteChanManager
}

// NewWorkerPool creates and starts a new worker pool
func NewWorkerPool(logger *slog.Logger, wsWriteChanManager services.WsWriteChanManager) services.WorkerPool {
	pool := &workerPool{
		jobQueue:           make(chan messageJob, jobQueueSize),
		logger:             logger,
		wsWriteChanManager: wsWriteChanManager,
	}
	pool.start()
	return pool
}

type server struct {
	wsWriteChanManager services.WsWriteChanManager
	centSubscriber     services.CentralisedSubscriber
	wsBridgeFactory    func(centSubscriber services.CentralisedSubscriber, wsConnID string, conn *websocket.Conn) services.WebSocketBridge
	logger             *slog.Logger
	workerPool         services.WorkerPool
	nodeID             common.NodeID
}

func New(
	nodeID common.NodeID,
	centSubscriber services.CentralisedSubscriber,
	wsBridgeFactory func(centSubscriber services.CentralisedSubscriber, wsConnID string, conn *websocket.Conn) services.WebSocketBridge,
	wsWriteChanManager services.WsWriteChanManager,
	logger *slog.Logger) *server {

	server := &server{
		centSubscriber:     centSubscriber,
		wsBridgeFactory:    wsBridgeFactory,
		wsWriteChanManager: wsWriteChanManager,
		logger:             logger,
		nodeID:             nodeID,
	}

	// Initialize the worker pool
	server.workerPool = NewWorkerPool(logger, wsWriteChanManager)

	return server
}

// start initializes the worker pool with fixed number of workers
func (wp *workerPool) start() {
	for i := 0; i < maxWorkers; i++ {
		go wp.worker(i)
	}
	wp.logger.Info(fmt.Sprintf("Started worker pool with %d workers", maxWorkers))
}

// worker processes message write jobs from the job queue
func (wp *workerPool) worker(id int) {
	for job := range wp.jobQueue {
		// Check if connection was closed before attempting write
		select {
		case <-job.ctx.Done():
			// Connection closed, skip this message
			continue
		default:
			// Connection still active, proceed
		}

		// Record latency at start
		metrics.LatencyHist.WithLabelValues(metrics.Hostname, "ws_bridge_ws_msg_write_start").Observe(float64(time.Since(job.publishedTime).Milliseconds()))

		// Use the manager's WritePreparedMessage method which ensures thread-safe writes
		err := wp.wsWriteChanManager.WritePreparedMessage(job.wsConnID, job.preparedMsg)
		if err != nil {
			// Only log errors - avoid logging every successful write
			logger := slogctx.FromCtx(job.ctx).With("worker-id", id, "client-id", job.wsConnID, "msg-id", job.msgID)
			logger.ErrorContext(job.ctx, "error writing to websocket", "error", err)
			// Connection error - context will be cancelled by ServeHTTP defer
		} else {
			// Record latency at completion
			metrics.LatencyHist.WithLabelValues(metrics.Hostname, "ws_bridge_ws_msg_write_done").Observe(float64(time.Since(job.publishedTime).Milliseconds()))
		}
	}
}

// SubmitJob submits a message write job to the worker pool (non-blocking)
func (wp *workerPool) SubmitMessageJob(ctx context.Context, conn *websocket.Conn, preparedMsg *websocket.PreparedMessage, publishedTime time.Time, msgID string, wsConnID string) {
	job := messageJob{
		preparedMsg:   preparedMsg,
		publishedTime: publishedTime,
		msgID:         msgID,
		ctx:           ctx,
		wsConnID:      wsConnID,
	}

	// Non-blocking send to prevent fanout worker from blocking
	select {
	case wp.jobQueue <- job:
		// Job successfully queued
	default:
		// Queue full - log and drop this message to prevent blocking
		// In production, you might want to track this metric
		wp.logger.Warn("worker queue full, dropping message", "ws-conn-id", wsConnID, "msg-id", msgID)
	}
}

func (server *server) Start() error {
	http.HandleFunc("/ws", server.ServeHTTP)

	fmt.Println("server started at http://:8080")
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		return err
	}
	return nil
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // allow all connections (for demo)
	},
	// Optimized buffer sizes for 5k connections on 8GiB RAM
	ReadBufferSize:  4096, // 4KB per connection (down from 16KB default)
	WriteBufferSize: 4096, // 4KB per connection (down from 16KB default)
	// Total per connection: ~8KB buffers + ~16KB overhead = ~24KB
	// 5000 connections × 24KB = ~120MB for WS buffers
}

func (pooler *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Upgrade error:", err)
		return
	}

	metrics.WsConnections.WithLabelValues(string(pooler.nodeID)).Inc()

	wsConnID := uuid.New().String()

	pooler.wsWriteChanManager.SetConnectionForClientID(wsConnID, conn)
	ctx, cancel := context.WithCancel(r.Context())
	ctx = slogctx.NewCtx(ctx, pooler.logger)
	defer func() {
		cancel()
		pooler.centSubscriber.UnsubscribeAll(context.TODO(), wsConnID)
		pooler.wsWriteChanManager.DeleteClientID(wsConnID)
		conn.Close()
		metrics.WsConnections.WithLabelValues(string(pooler.nodeID)).Dec()
	}()

	// Configure WebSocket connection timeouts
	// conn.SetReadDeadline(time.Now().Add(pongWait))
	// conn.SetPongHandler(func(string) error {
	// 	conn.SetReadDeadline(time.Now().Add(pongWait))
	// 	return nil
	// })
	bridge := pooler.wsBridgeFactory(pooler.centSubscriber, wsConnID, conn)

	// Start ping ticker in a goroutine
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()

	// Only ProcessMessagesFromClient blocks - no more ProcessMessagesFromServer goroutine needed!
	bridge.ProcessMessagesFromClient(ctx)
}
