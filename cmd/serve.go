package cmd

import (
	"fmt"
	"log"

	"github.com/kychandar/ottam/config"
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
	// your main app logic here
}
