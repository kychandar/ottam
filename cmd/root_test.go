package cmd

import (
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAsyncLogger(t *testing.T) {
	logger, cleanup := NewAsyncLogger()
	defer cleanup()

	assert.NotNil(t, logger)
	assert.IsType(t, &slog.Logger{}, logger)
}

func TestSetupLogger(t *testing.T) {
	logger, cleanup := SetupLogger()
	defer cleanup()

	assert.NotNil(t, logger)
	assert.IsType(t, &slog.Logger{}, logger)
}

func TestLogger_CanLog(t *testing.T) {
	logger, cleanup := NewAsyncLogger()
	defer cleanup()

	// Test that logging doesn't panic
	assert.NotPanics(t, func() {
		logger.Info("test info message")
		logger.Warn("test warn message")
		logger.Error("test error message")
	})
}

func TestLogger_WithFields(t *testing.T) {
	logger, cleanup := NewAsyncLogger()
	defer cleanup()

	// Test logging with fields
	assert.NotPanics(t, func() {
		logger.With("key", "value").Info("test message with fields")
		logger.With("id", 123, "name", "test").Warn("multiple fields")
	})
}

func TestRootCmd_Exists(t *testing.T) {
	assert.NotNil(t, rootCmd)
	assert.Equal(t, "processor", rootCmd.Use)
	assert.NotEmpty(t, rootCmd.Short)
}

func TestRootCmd_Flags(t *testing.T) {
	flag := rootCmd.PersistentFlags().Lookup("config")
	require.NotNil(t, flag)
	assert.Equal(t, "config", flag.Name)
}

func TestInitConfig_WithoutFile(t *testing.T) {
	// Test that initConfig doesn't panic
	assert.NotPanics(t, func() {
		initConfig()
	})
}

func TestInitConfig_WithFile(t *testing.T) {
	// Create a temporary config file
	tmpFile, err := os.CreateTemp("", "test-config-*.yaml")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	// Write some config data
	_, err = tmpFile.WriteString("server:\n  host: localhost\n  port: 8080\n")
	require.NoError(t, err)
	tmpFile.Close()

	// Set cfgFile and test
	originalCfgFile := cfgFile
	cfgFile = tmpFile.Name()
	defer func() { cfgFile = originalCfgFile }()

	assert.NotPanics(t, func() {
		initConfig()
	})
}

func TestServeCmd_Exists(t *testing.T) {
	assert.NotNil(t, serveCmd)
	assert.Equal(t, "server", serveCmd.Use)
	assert.NotEmpty(t, serveCmd.Short)
	assert.NotNil(t, serveCmd.Run)
}

func TestCmdVariables(t *testing.T) {
	t.Run("cfgFile variable exists", func(t *testing.T) {
		// Just verify we can access these variables
		_ = cfgFile
	})

	t.Run("env variable exists", func(t *testing.T) {
		_ = env
	})
}

func TestLogger_Cleanup(t *testing.T) {
	logger, cleanup := NewAsyncLogger()
	assert.NotNil(t, logger)

	// Test that cleanup doesn't panic
	assert.NotPanics(t, func() {
		cleanup()
	})
}

func TestLogger_MultipleInstances(t *testing.T) {
	logger1, cleanup1 := NewAsyncLogger()
	defer cleanup1()
	
	logger2, cleanup2 := NewAsyncLogger()
	defer cleanup2()

	assert.NotNil(t, logger1)
	assert.NotNil(t, logger2)
	
	// Both should be able to log
	logger1.Info("logger 1")
	logger2.Info("logger 2")
}

func TestLogger_LogLevels(t *testing.T) {
	logger, cleanup := NewAsyncLogger()
	defer cleanup()

	tests := []struct {
		name string
		fn   func()
	}{
		{"Info", func() { logger.Info("info") }},
		{"Warn", func() { logger.Warn("warn") }},
		{"Error", func() { logger.Error("error") }},
		{"Debug", func() { logger.Debug("debug") }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotPanics(t, tt.fn)
		})
	}
}

func TestLogger_WithContext(t *testing.T) {
	logger, cleanup := NewAsyncLogger()
	defer cleanup()

	contextLogger := logger.With(
		"service", "test",
		"version", "1.0",
		"env", "test",
	)

	assert.NotNil(t, contextLogger)
	assert.NotPanics(t, func() {
		contextLogger.Info("message with context")
	})
}

func TestLogger_WithGroup(t *testing.T) {
	logger, cleanup := NewAsyncLogger()
	defer cleanup()

	groupLogger := logger.WithGroup("test-group")
	assert.NotNil(t, groupLogger)
	
	assert.NotPanics(t, func() {
		groupLogger.Info("grouped message")
	})
}

func TestSetupLogger_ReturnsValidLogger(t *testing.T) {
	logger, cleanup := SetupLogger()
	defer cleanup()

	// Verify logger can be used
	assert.NotPanics(t, func() {
		logger.Info("testing setup logger")
		logger.With("test", "value").Warn("with fields")
	})
}

