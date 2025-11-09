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
	maxWorkers   = 400
	jobQueueSize = 10000

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

		logger := slogctx.FromCtx(job.ctx).With("worker-id", id, "client-id", job.wsConnID, "msg-id", job.msgID)

		logger.InfoContext(job.ctx, "worker processing message write", "ms", time.Since(job.publishedTime).String())
		metrics.LatencyHist.WithLabelValues(metrics.Hostname, "ws_bridge_ws_msg_write_start").Observe(float64(time.Since(job.publishedTime).Milliseconds()))

		// Use the manager's WritePreparedMessage method which ensures thread-safe writes
		err := wp.wsWriteChanManager.WritePreparedMessage(job.wsConnID, job.preparedMsg)
		if err != nil {
			logger.ErrorContext(job.ctx, "error writing to websocket", "error", err)
			// Connection error - context will be cancelled by ServeHTTP defer
		} else {
			metrics.LatencyHist.WithLabelValues(metrics.Hostname, "ws_bridge_ws_msg_write_done").Observe(float64(time.Since(job.publishedTime).Milliseconds()))
			logger.InfoContext(job.ctx, "worker finished writing message to websocket", "ms", time.Since(job.publishedTime).String())
		}
	}
}

// SubmitJob submits a message write job to the worker pool
func (wp *workerPool) SubmitMessageJob(ctx context.Context, conn *websocket.Conn, preparedMsg *websocket.PreparedMessage, publishedTime time.Time, msgID string, wsConnID string) {
	wp.jobQueue <- messageJob{
		preparedMsg:   preparedMsg,
		publishedTime: publishedTime,
		msgID:         msgID,
		ctx:           ctx,
		wsConnID:      wsConnID,
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
