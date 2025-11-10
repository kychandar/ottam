package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"runtime"
	"strconv"

	"github.com/kychandar/ottam/common"
	"github.com/kychandar/ottam/config"
	"github.com/kychandar/ottam/ds"
	"github.com/kychandar/ottam/http"
	"github.com/kychandar/ottam/services"
	"github.com/kychandar/ottam/services/centralisedSubscriber"
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
	// Configure GOMAXPROCS for optimal CPU usage
	// Default to 4 cores (optimized for 4 CPU / 8GiB / 5k connections)
	// Can be overridden with GOMAXPROCS env var
	if gomaxprocs := os.Getenv("GOMAXPROCS"); gomaxprocs == "" {
		runtime.GOMAXPROCS(4)
		fmt.Println("GOMAXPROCS set to 4 (default for 4 CPU pods)")
	} else {
		if n, err := strconv.Atoi(gomaxprocs); err == nil {
			runtime.GOMAXPROCS(n)
			fmt.Printf("GOMAXPROCS set to %d (from env)\n", n)
		}
	}
	fmt.Printf("Using %d CPU cores\n", runtime.GOMAXPROCS(0))
	
	// Configure GC for lower memory footprint
	// Trigger GC when heap grows by 100% (default is 100, we keep it)
	// For memory-constrained environments, could lower to 75-80
	fmt.Printf("GOGC: %s (100 = default, lower = more frequent GC)\n", os.Getenv("GOGC"))

	fmt.Printf("Server starting on %s:%d\n", cfg.Server.Host, cfg.Server.Port)
	logger, cleanup := SetupLogger()
	defer cleanup()
	ctx := slogctx.NewCtx(context.Background(), logger)

	wswritechannelmanager := wswritechannelmanager.NewClientWriterManager()
	hostName, err := os.Hostname()
	if err != nil {
		panic(err)
	}

	fmt.Println("hostName", hostName)
	pubSubProvider, err := pubSubProvider.NewNatsPubSub(cfg.PubSub.URL)
	if err != nil {
		panic(err)
	}

	// Create worker pool first so it can be shared with centralisedSubscriber
	workerPool := http.NewWorkerPool(logger, wswritechannelmanager)

	centSubs := centralisedSubscriber.New(common.NodeID(hostName), wswritechannelmanager, workerPool, func() services.SerializableMessage {
		return ds.NewEmpty()
	}, pubSubProvider)

	err = centSubs.ProcessDownstreamMessages(ctx)
	if err != nil {
		panic(err)
	}

	go centSubs.SubscriptionSyncer(ctx)

	server := http.New(common.NodeID(hostName), centSubs, websocketbridge.NewWsBridgeFactory(), wswritechannelmanager, logger)
	err = server.Start()
	panic(err)
}
