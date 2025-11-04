package cmd

import (
	"fmt"
	"log"
	"os"

	"github.com/kychandar/ottam/config"
	"github.com/kychandar/ottam/http"
	"github.com/kychandar/ottam/services/centralisedSubscriber"
	valkey "github.com/kychandar/ottam/services/inMemCache/valKey"
	websocketbridge "github.com/kychandar/ottam/services/websocketBridge"
	"github.com/spf13/cobra"
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

	// TODO: fix below
	cache, err := valkey.NewValkeySetCache(cfg)
	if err != nil {
		panic(err)
	}
	hostNmae, err := os.Hostname()
	centSubs := centralisedSubscriber.New(cache, hostNmae)
	server := http.New(centSubs, websocketbridge.NewWsBridgeFactory())
	server.Start()
}
