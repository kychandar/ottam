package cmd

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	slogzap "github.com/samber/slog-zap/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	cfgFile string
	env     string
	rootCmd = &cobra.Command{
		Use:   "processor",
		Short: "Async message processor that helps in fanning out the messages",
	}
)

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default: config/config.yaml)")
	cobra.OnInitialize(initConfig)
}

func initConfig() {
	if cfgFile != "" {
		// Use the specified config file
		viper.SetConfigFile(cfgFile)
	} else {
		// Default location: $HOME/.myapp.yaml
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		viper.AddConfigPath(home)
		viper.SetConfigType("yaml")
		viper.SetConfigName(".myapp")
	}

	viper.AutomaticEnv() // read in environment variables that match

	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}

func NewAsyncLogger() (*slog.Logger, func()) {
	// Lumberjack for file rotation
	fileWriter := &lumberjack.Logger{
		Filename:   "app.log",
		MaxSize:    100, // MB
		MaxBackups: 3,
		MaxAge:     28, // days
		Compress:   true,
	}

	// Console writer
	consoleWriter := zapcore.AddSync(os.Stdout)

	// Encoder config
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder

	// Create encoders
	fileEncoder := zapcore.NewJSONEncoder(encoderConfig)
	consoleEncoder := zapcore.NewConsoleEncoder(encoderConfig)

	// CRITICAL: Wrap writers with BufferedWriteSyncer for async writes
	bufferedFileWriter := &zapcore.BufferedWriteSyncer{
		WS:            zapcore.AddSync(fileWriter),
		Size:          256 * 1024, // 256KB buffer
		FlushInterval: 5 * time.Second,
	}

	bufferedConsoleWriter := &zapcore.BufferedWriteSyncer{
		WS:            consoleWriter,
		Size:          64 * 1024, // 64KB buffer
		FlushInterval: 1 * time.Second,
	}

	// Create core with buffered writers
	core := zapcore.NewTee(
		zapcore.NewCore(fileEncoder, bufferedFileWriter, zapcore.InfoLevel),
		zapcore.NewCore(consoleEncoder, bufferedConsoleWriter, zapcore.ErrorLevel),
	)

	// Build zap logger
	zapLogger := zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))
	// Convert to slog
	handler := slogzap.Option{
		Level:  slog.LevelInfo, // Change to Info for production
		Logger: zapLogger,
	}.NewZapHandler()

	return slog.New(handler), func() { zapLogger.Sync() }
}

// Setup logger in main
func SetupLogger() (*slog.Logger, func()) {
	return NewAsyncLogger()
}
