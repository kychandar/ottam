package cmd

import (
	"context"
	"log"
	"log/slog"
	"time"

	"github.com/kychandar/ottam/config"
	"github.com/kychandar/ottam/ds"
	"github.com/kychandar/ottam/services"
	centprocessor "github.com/kychandar/ottam/services/centProcessor"
	valkey "github.com/kychandar/ottam/services/inMemCache/valKey"
	pubSubProvider "github.com/kychandar/ottam/services/pubsub/nats"
	slogzap "github.com/samber/slog-zap/v2"
	"github.com/spf13/cobra"
	slogctx "github.com/veqryn/slog-context"
	"go.uber.org/zap"
)

var centProcessorServeCmd = &cobra.Command{
	Use:   "centprocessor",
	Short: "Start the centprocessor",
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.Load(cfgFile, env)
		if err != nil {
			log.Fatalf("failed to load config: %v", err)
		}

		startCentProcessorServeCmd(cfg)
	},
}

func init() {
	rootCmd.AddCommand(centProcessorServeCmd)
}

func startCentProcessorServeCmd(cfg *config.Config) {
	zapLogger, _ := zap.NewProduction()
	logger := slog.New(slogzap.Option{Level: slog.LevelDebug, Logger: zapLogger}.NewZapHandler())
	ctx := slogctx.NewCtx(context.Background(), logger)
	pubSubProvider, err := pubSubProvider.NewNatsPubSub(cfg.PubSub.URL)
	if err != nil {
		panic(err)
	}

	dataStore, err := valkey.NewValkeySetCache(cfg)
	if err != nil {
		panic(err)
	}

	centProcessor := centprocessor.NewCentralProcessor(context.TODO(), pubSubProvider, dataStore, func() services.SerializableMessage {
		return ds.NewEmpty()
	})

	err = centProcessor.Start(ctx)
	if err != nil {
		panic(err)
	}
	time.Sleep(100000 * time.Hour)
}
