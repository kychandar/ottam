package cmd

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/kychandar/ottam/common"
	"github.com/kychandar/ottam/config"
	"github.com/kychandar/ottam/ds"
	"github.com/kychandar/ottam/http"
	"github.com/kychandar/ottam/services"
	"github.com/kychandar/ottam/services/centralisedSubscriber"
	valkey "github.com/kychandar/ottam/services/inMemCache/valKey"
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
	fmt.Printf("Server starting on %s:%d\n", cfg.Server.Host, cfg.Server.Port)
	logger, cleanup := SetupLogger()
	defer cleanup()
	ctx := slogctx.NewCtx(context.Background(), logger)
	// TODO: fix below
	dataStore, err := valkey.NewValkeySetCache(cfg)
	if err != nil {
		panic(err)
	}
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
	centSubs := centralisedSubscriber.New(dataStore, common.NodeID("node1"), wswritechannelmanager, func() services.SerializableMessage {
		return ds.NewEmpty()
	}, pubSubProvider)
	err = centSubs.ProcessDownstreamMessages(ctx)
	if err != nil {
		panic(err)
	}

	server := http.New(centSubs, websocketbridge.NewWsBridgeFactory(), wswritechannelmanager, logger)
	server.Start()
}
