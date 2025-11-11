package http

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/kychandar/ottam/config"
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
	metricsRegistry    services.MetricsRegistry
}

// NewWorkerPool creates and starts a new worker pool
func NewWorkerPool(logger *slog.Logger,
	wsWriteChanManager services.WsWriteChanManager,
	metricsRegistry services.MetricsRegistry,
) services.WorkerPool {
	pool := &workerPool{
		jobQueue:           make(chan messageJob, jobQueueSize),
		logger:             logger,
		wsWriteChanManager: wsWriteChanManager,
		metricsRegistry:    metricsRegistry,
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
	metricsRegistry    services.MetricsRegistry
	httpServer         *http.Server
	config             *config.Config
	healthChecker      *HealthChecker
	shutdownWg         sync.WaitGroup
}

func New(
	centSubscriber services.CentralisedSubscriber,
	wsBridgeFactory func(centSubscriber services.CentralisedSubscriber, wsConnID string, conn *websocket.Conn) services.WebSocketBridge,
	wsWriteChanManager services.WsWriteChanManager,
	metricsRegistry services.MetricsRegistry,
	logger *slog.Logger,
	cfg *config.Config) *server {

	server := &server{
		centSubscriber:     centSubscriber,
		wsBridgeFactory:    wsBridgeFactory,
		wsWriteChanManager: wsWriteChanManager,
		logger:             logger,
		metricsRegistry:    metricsRegistry,
		config:             cfg,
		healthChecker:      NewHealthChecker(logger, "1.0.0"),
	}

	// Initialize the worker pool
	server.workerPool = NewWorkerPool(logger, wsWriteChanManager, metricsRegistry)

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
		wp.metricsRegistry.ObserveWsBridgeWriteStart(job.publishedTime)

		// Use the manager's WritePreparedMessage method which ensures thread-safe writes
		err := wp.wsWriteChanManager.WritePreparedMessage(job.wsConnID, job.preparedMsg)
		if err != nil {
			// Only log errors - avoid logging every successful write
			logger := slogctx.FromCtx(job.ctx).With("worker-id", id, "client-id", job.wsConnID, "msg-id", job.msgID)
			logger.ErrorContext(job.ctx, "error writing to websocket", "error", err)
			// Connection error - context will be cancelled by ServeHTTP defer
		} else {
			// Record latency at completion
			wp.metricsRegistry.ObserveWsBridgeWriteEnd(job.publishedTime)
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

func (server *server) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", server.ServeHTTP)

	addr := fmt.Sprintf("%s:%d", server.config.Server.Host, server.config.Server.Port)
	
	server.httpServer = &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	// Configure TLS if enabled
	if server.config.Server.TLS.Enabled {
		tlsConfig, err := server.loadTLSConfig()
		if err != nil {
			return fmt.Errorf("failed to load TLS config: %w", err)
		}
		server.httpServer.TLSConfig = tlsConfig
		server.logger.Info("TLS enabled for HTTP server")
	}

	// Mark service as ready
	server.healthChecker.SetReady(true)

	server.shutdownWg.Add(1)
	go func() {
		defer server.shutdownWg.Done()
		<-ctx.Done()
		server.logger.Info("Shutting down HTTP server...")
		server.healthChecker.SetReady(false)
		
		shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Duration(server.config.Server.ShutdownTimeout)*time.Second)
		defer cancel()
		
		if err := server.httpServer.Shutdown(shutdownCtx); err != nil {
			server.logger.Error("HTTP server shutdown error", "error", err)
		}
	}()

	server.logger.Info("Starting HTTP server", "address", addr, "tls", server.config.Server.TLS.Enabled)

	var err error
	if server.config.Server.TLS.Enabled {
		err = server.httpServer.ListenAndServeTLS(
			server.config.Server.TLS.CertFile,
			server.config.Server.TLS.KeyFile,
		)
	} else {
		err = server.httpServer.ListenAndServe()
	}

	if err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("HTTP server error: %w", err)
	}

	return nil
}

// loadTLSConfig loads TLS configuration for the HTTP server
func (server *server) loadTLSConfig() (*tls.Config, error) {
	return &tls.Config{
		MinVersion: tls.VersionTLS12,
		CurvePreferences: []tls.CurveID{
			tls.CurveP256,
			tls.X25519,
		},
		PreferServerCipherSuites: true,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		},
	}, nil
}

// Shutdown gracefully shuts down the server
func (server *server) Shutdown(ctx context.Context) error {
	server.logger.Info("Initiating graceful shutdown...")
	server.healthChecker.SetReady(false)
	
	shutdownCtx, cancel := context.WithTimeout(ctx, time.Duration(server.config.Server.ShutdownTimeout)*time.Second)
	defer cancel()

	if server.httpServer != nil {
		if err := server.httpServer.Shutdown(shutdownCtx); err != nil {
			server.logger.Error("Error shutting down HTTP server", "error", err)
			return err
		}
	}

	// Wait for all goroutines to finish
	done := make(chan struct{})
	go func() {
		server.shutdownWg.Wait()
		close(done)
	}()

	select {
	case <-done:
		server.logger.Info("Graceful shutdown completed")
		return nil
	case <-shutdownCtx.Done():
		server.logger.Warn("Shutdown timeout exceeded, forcing shutdown")
		return shutdownCtx.Err()
	}
}

// GetHealthChecker returns the health checker instance
func (server *server) GetHealthChecker() *HealthChecker {
	return server.healthChecker
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

func (server *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Upgrade error:", err)
		return
	}

	server.metricsRegistry.IncWsConnectionCount()
	wsConnID := uuid.New().String()

	server.wsWriteChanManager.SetConnectionForClientID(wsConnID, conn)
	ctx, cancel := context.WithCancel(r.Context())
	ctx = slogctx.NewCtx(ctx, server.logger)
	defer func() {
		cancel()
		server.centSubscriber.UnsubscribeAll(context.TODO(), wsConnID)
		server.wsWriteChanManager.DeleteClientID(wsConnID)
		conn.Close()
		server.metricsRegistry.DecWsConnectionCount()
	}()

	// Configure WebSocket connection timeouts
	// conn.SetReadDeadline(time.Now().Add(pongWait))
	// conn.SetPongHandler(func(string) error {
	// 	conn.SetReadDeadline(time.Now().Add(pongWait))
	// 	return nil
	// })
	bridge := server.wsBridgeFactory(server.centSubscriber, wsConnID, conn)

	// Start ping ticker in a goroutine
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()

	// Only ProcessMessagesFromClient blocks - no more ProcessMessagesFromServer goroutine needed!
	bridge.ProcessMessagesFromClient(ctx)
}
