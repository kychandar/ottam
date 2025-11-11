package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_Structure(t *testing.T) {
	t.Run("config struct creation", func(t *testing.T) {
		cfg := &Config{}
		cfg.Server.Host = "localhost"
		cfg.Server.Port = 8080
		cfg.PubSub.Provider = "nats"
		cfg.PubSub.URL = "nats://localhost:4222"
		cfg.RedisProtoCache.Addr = []string{"localhost:6379"}

		assert.Equal(t, "localhost", cfg.Server.Host)
		assert.Equal(t, 8080, cfg.Server.Port)
		assert.Equal(t, "nats", cfg.PubSub.Provider)
		assert.Equal(t, "nats://localhost:4222", cfg.PubSub.URL)
		assert.Equal(t, []string{"localhost:6379"}, cfg.RedisProtoCache.Addr)
	})
}

func TestLoad_WithConfigFile(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "ottam-config-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create test config file
	configContent := `
server:
  host: "127.0.0.1"
  port: 9090
pubsub:
  provider: "nats"
  url: "nats://test:4222"
redisprotocache:
  addr:
    - "redis1:6379"
    - "redis2:6379"
`
	configPath := filepath.Join(tmpDir, "config.yaml")
	err = os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	// Load config
	cfg, err := Load(configPath, "")
	require.NoError(t, err)
	assert.NotNil(t, cfg)

	// Verify values
	assert.Equal(t, "127.0.0.1", cfg.Server.Host)
	assert.Equal(t, 9090, cfg.Server.Port)
	assert.Equal(t, "nats", cfg.PubSub.Provider)
	assert.Equal(t, "nats://test:4222", cfg.PubSub.URL)
	assert.Equal(t, []string{"redis1:6379", "redis2:6379"}, cfg.RedisProtoCache.Addr)
}

func TestLoad_WithDefaults(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "ottam-config-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create minimal config file (should use defaults)
	configContent := `
pubsub:
  provider: "nats"
  url: "nats://localhost:4222"
`
	configPath := filepath.Join(tmpDir, "config.yaml")
	err = os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	// Load config
	cfg, err := Load(configPath, "")
	require.NoError(t, err)
	assert.NotNil(t, cfg)

	// Verify defaults are applied
	assert.Equal(t, "0.0.0.0", cfg.Server.Host)
	assert.Equal(t, 8080, cfg.Server.Port)
	assert.Equal(t, "nats", cfg.PubSub.Provider)
}

func TestLoad_WithEnvironmentOverride(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "ottam-config-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create base config
	configContent := `
server:
  host: "localhost"
  port: 8080
pubsub:
  provider: "nats"
  url: "nats://localhost:4222"
`
	configPath := filepath.Join(tmpDir, "config.yaml")
	err = os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	// Create environment-specific config
	envConfigContent := `
server:
  port: 9090
pubsub:
  url: "nats://prod:4222"
`
	envConfigPath := filepath.Join(tmpDir, "config.prod.yaml")
	err = os.WriteFile(envConfigPath, []byte(envConfigContent), 0644)
	require.NoError(t, err)

	// Save current working directory and change to temp dir
	originalWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(originalWd)

	// Load config with environment - don't pass absolute path so it can find env-specific config
	cfg, err := Load("", "prod")
	require.NoError(t, err)
	assert.NotNil(t, cfg)

	// Verify environment overrides are applied
	assert.Equal(t, "localhost", cfg.Server.Host) // not overridden
	assert.Equal(t, 9090, cfg.Server.Port)        // overridden
	assert.Equal(t, "nats://prod:4222", cfg.PubSub.URL) // overridden
}

func TestLoad_NonExistentConfigFile(t *testing.T) {
	cfg, err := Load("/nonexistent/config.yaml", "")
	assert.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "error reading config")
}

func TestLoad_InvalidYAML(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "ottam-config-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create invalid YAML file
	invalidContent := `
server:
  host: "localhost"
  port: invalid_port
  this is not valid yaml
`
	configPath := filepath.Join(tmpDir, "config.yaml")
	err = os.WriteFile(configPath, []byte(invalidContent), 0644)
	require.NoError(t, err)

	// Try to load config
	cfg, err := Load(configPath, "")
	assert.Error(t, err)
	assert.Nil(t, cfg)
}

func TestLoad_EmptyConfig(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "ottam-config-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create empty config file
	configPath := filepath.Join(tmpDir, "config.yaml")
	err = os.WriteFile(configPath, []byte(""), 0644)
	require.NoError(t, err)

	// Load config - should work with defaults
	cfg, err := Load(configPath, "")
	require.NoError(t, err)
	assert.NotNil(t, cfg)

	// Verify defaults
	assert.Equal(t, "0.0.0.0", cfg.Server.Host)
	assert.Equal(t, 8080, cfg.Server.Port)
}

func TestLoad_WithoutConfigFileArg(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "ottam-config-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create config file in current directory
	configContent := `
server:
  host: "localhost"
  port: 7070
pubsub:
  provider: "nats"
  url: "nats://localhost:4222"
`
	configPath := filepath.Join(tmpDir, "config.yaml")
	err = os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	// Save current working directory and change to temp dir
	originalWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(originalWd)

	// Load config without specifying file path
	cfg, err := Load("", "")
	require.NoError(t, err)
	assert.NotNil(t, cfg)

	// Verify values
	assert.Equal(t, "localhost", cfg.Server.Host)
	assert.Equal(t, 7070, cfg.Server.Port)
}

func TestLoad_EnvironmentVariableOverride(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "ottam-config-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create config file
	configContent := `
server:
  host: "localhost"
  port: 8080
pubsub:
  provider: "nats"
  url: "nats://localhost:4222"
`
	configPath := filepath.Join(tmpDir, "config.yaml")
	err = os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	// Set environment variable
	os.Setenv("MYAPP_SERVER_PORT", "3000")
	defer os.Unsetenv("MYAPP_SERVER_PORT")

	// Load config
	cfg, err := Load(configPath, "")
	require.NoError(t, err)
	assert.NotNil(t, cfg)

	// Verify environment variable override
	assert.Equal(t, 3000, cfg.Server.Port)
	assert.Equal(t, "localhost", cfg.Server.Host)
}

func TestConfig_RedisProtoCacheMultipleAddresses(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "ottam-config-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create config with multiple Redis addresses
	configContent := `
server:
  host: "localhost"
  port: 8080
pubsub:
  provider: "nats"
  url: "nats://localhost:4222"
redisprotocache:
  addr:
    - "redis1.cluster:6379"
    - "redis2.cluster:6379"
    - "redis3.cluster:6379"
`
	configPath := filepath.Join(tmpDir, "config.yaml")
	err = os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	// Load config
	cfg, err := Load(configPath, "")
	require.NoError(t, err)
	assert.NotNil(t, cfg)

	// Verify multiple addresses
	assert.Len(t, cfg.RedisProtoCache.Addr, 3)
	assert.Contains(t, cfg.RedisProtoCache.Addr, "redis1.cluster:6379")
	assert.Contains(t, cfg.RedisProtoCache.Addr, "redis2.cluster:6379")
	assert.Contains(t, cfg.RedisProtoCache.Addr, "redis3.cluster:6379")
}

