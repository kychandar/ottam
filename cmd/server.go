package cmd

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"syscall"
	"time"

	"github.com/kychandar/ottam/common"
	"github.com/kychandar/ottam/common/ds"
	"github.com/kychandar/ottam/config"
	httpserver "github.com/kychandar/ottam/http"
	"github.com/kychandar/ottam/services"
	"github.com/kychandar/ottam/services/centralisedSubscriber"
	metricsregistry "github.com/kychandar/ottam/services/metricsRegistry"
	pubSubProvider "github.com/kychandar/ottam/services/pubsub/nats"
	websocketbridge "github.com/kychandar/ottam/services/websocketBridge"
	wswritechannelmanager "github.com/kychandar/ottam/services/wsWriteChanManager"
	"github.com/spf13/cobra"
	slogctx "github.com/veqryn/slog-context"
)

var serveCmd = &cobra.Command{
	Use:   "server",
	Short: "Start the real-time backend server",
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.Load(cfgFile, env)
		if err != nil {
			log.Fatalf("failed to load config: %v", err)
		}

		startServer(cfg)
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
}

func startServer(cfg *config.Config) {
	logger, cleanup := SetupLogger()
	defer cleanup()
	
	// Configure GOMAXPROCS for optimal CPU usage
	// Default to 4 cores (optimized for 4 CPU / 8GiB / 5k connections)
	// Can be overridden with GOMAXPROCS env var
	if gomaxprocs := os.Getenv("GOMAXPROCS"); gomaxprocs == "" {
		runtime.GOMAXPROCS(4)
		logger.Info("GOMAXPROCS set to 4 (default for 4 CPU pods)")
	} else {
		if n, err := strconv.Atoi(gomaxprocs); err == nil {
			runtime.GOMAXPROCS(n)
			logger.Info(fmt.Sprintf("GOMAXPROCS set to %d (from env)", n))
		}
	}
	logger.Info(fmt.Sprintf("Using %d CPU cores", runtime.GOMAXPROCS(0)))

	logger.Info("Server starting", "host", cfg.Server.Host, "port", cfg.Server.Port)

	// Create root context for the application
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx = slogctx.NewCtx(ctx, logger)

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	// Get hostname for instance identification
	hostName, err := os.Hostname()
	if err != nil {
		logger.Error("error in fetching hostname", "err", err)
		os.Exit(1)
	}

	// Initialize PubSub provider with TLS support
	var pubSubProviderInst services.PubSubProvider
	if cfg.PubSub.TLS.Enabled {
		pubSubProviderInst, err = pubSubProvider.NewNatsPubSubWithTLS(cfg.PubSub.URL, cfg.PubSub.TLS)
		if err != nil {
			logger.Error("error in creating pubsub provider with TLS", "err", err)
			os.Exit(1)
		}
		logger.Info("NATS connection established with TLS")
	} else {
		pubSubProviderInst, err = pubSubProvider.NewNatsPubSub(cfg.PubSub.URL)
		if err != nil {
			logger.Error("error in creating pubsub provider", "err", err)
			os.Exit(1)
		}
		logger.Info("NATS connection established without TLS")
	}

	// Initialize components
	wswritechannelmanager := wswritechannelmanager.NewClientWriterManager()
	metricsReg := metricsregistry.New(hostName)

	// Create worker pool first so it can be shared with centralisedSubscriber
	workerPool := httpserver.NewWorkerPool(logger, wswritechannelmanager, metricsReg)

	centSubs := centralisedSubscriber.New(common.NodeID(hostName), wswritechannelmanager, workerPool, func() services.SerializableMessage {
		return ds.NewEmpty()
	}, pubSubProviderInst)

	err = centSubs.ProcessDownstreamMessages(ctx)
	if err != nil {
		logger.Error("error in starting downstream processor", "err", err)
		os.Exit(1)
	}

	go centSubs.SubscriptionSyncer(ctx)

	// Initialize HTTP server
	server := httpserver.New(centSubs, websocketbridge.NewWsBridgeFactory(), wswritechannelmanager, metricsReg, logger, cfg)

	// Start health check server if enabled
	var healthServer *http.Server
	if cfg.Health.Enabled {
		healthServer = startHealthServer(cfg, server.GetHealthChecker(), logger)
	}

	// Channel to signal when server is done
	serverDone := make(chan error, 1)

	// Start HTTP server in a goroutine
	go func() {
		logger.Info("Starting main HTTP/WebSocket server...")
		if err := server.Start(ctx); err != nil {
			serverDone <- err
		}
		close(serverDone)
	}()

	// Wait for shutdown signal or server error
	select {
	case sig := <-sigChan:
		logger.Info("Received shutdown signal", "signal", sig.String())
		cancel() // Cancel context to signal all components to shutdown
		
		// Perform graceful shutdown
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), time.Duration(cfg.Server.ShutdownTimeout)*time.Second)
		defer shutdownCancel()

		// Shutdown HTTP server
		logger.Info("Shutting down HTTP server...")
		if err := server.Shutdown(shutdownCtx); err != nil {
			logger.Error("Error during server shutdown", "error", err)
		}

		// Shutdown health server
		if healthServer != nil {
			logger.Info("Shutting down health check server...")
			if err := healthServer.Shutdown(shutdownCtx); err != nil {
				logger.Error("Error during health server shutdown", "error", err)
			}
		}

		// Close PubSub connection
		logger.Info("Closing PubSub connection...")
		if err := pubSubProviderInst.Close(); err != nil {
			logger.Error("Error closing PubSub connection", "error", err)
		}

		logger.Info("Graceful shutdown completed successfully")

	case err := <-serverDone:
		if err != nil {
			logger.Error("Server error", "error", err)
			os.Exit(1)
		}
	}
}

// startHealthServer starts the health check HTTP server
func startHealthServer(cfg *config.Config, healthChecker *httpserver.HealthChecker, logger *slog.Logger) *http.Server {
	// Use the default http.ServeMux which has metrics and pprof handlers registered
	mux := http.DefaultServeMux
	mux.HandleFunc(cfg.Health.ReadinessPath, healthChecker.ReadinessHandler())
	mux.HandleFunc(cfg.Health.LivenessPath, healthChecker.LivenessHandler())

	addr := fmt.Sprintf(":%d", cfg.Health.Port)
	server := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		logger.Info("Starting health check server", "address", addr, 
			"readiness", cfg.Health.ReadinessPath, 
			"liveness", cfg.Health.LivenessPath,
			"metrics", "/metrics",
			"pprof", "/debug/pprof")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Health server error", "error", err)
		}
	}()

	return server
}
