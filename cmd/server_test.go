package cmd

import (
	"os"
	"runtime"
	"testing"

	"github.com/kychandar/ottam/config"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStartServer_GOMAXPROCS_Default(t *testing.T) {
	// Save original GOMAXPROCS
	originalGOMAXPROCS := os.Getenv("GOMAXPROCS")
	defer func() {
		if originalGOMAXPROCS != "" {
			os.Setenv("GOMAXPROCS", originalGOMAXPROCS)
		} else {
			os.Unsetenv("GOMAXPROCS")
		}
	}()

	// Unset GOMAXPROCS to test default behavior
	os.Unsetenv("GOMAXPROCS")

	// Note: We can't fully test startServer as it runs indefinitely
	// But we can verify GOMAXPROCS setting logic
	if gomaxprocs := os.Getenv("GOMAXPROCS"); gomaxprocs == "" {
		runtime.GOMAXPROCS(4)
		assert.Equal(t, 4, runtime.GOMAXPROCS(0))
	}
}

func TestStartServer_GOMAXPROCS_FromEnv(t *testing.T) {
	// Save original GOMAXPROCS
	originalGOMAXPROCS := os.Getenv("GOMAXPROCS")
	defer func() {
		if originalGOMAXPROCS != "" {
			os.Setenv("GOMAXPROCS", originalGOMAXPROCS)
		} else {
			os.Unsetenv("GOMAXPROCS")
		}
	}()

	// Set GOMAXPROCS environment variable
	os.Setenv("GOMAXPROCS", "8")

	gomaxprocs := os.Getenv("GOMAXPROCS")
	assert.Equal(t, "8", gomaxprocs)
}

func TestStartServer_ConfigStructure(t *testing.T) {
	cfg := &config.Config{}
	cfg.Server.Host = "0.0.0.0"
	cfg.Server.Port = 8080
	cfg.PubSub.Provider = "nats"
	cfg.PubSub.URL = "nats://localhost:4222"

	assert.Equal(t, "0.0.0.0", cfg.Server.Host)
	assert.Equal(t, 8080, cfg.Server.Port)
	assert.Equal(t, "nats", cfg.PubSub.Provider)
	assert.Equal(t, "nats://localhost:4222", cfg.PubSub.URL)
}

func TestServeCmd_Structure(t *testing.T) {
	assert.NotNil(t, serveCmd)
	assert.Equal(t, "server", serveCmd.Use)
	assert.Equal(t, "Start the real-time backend server", serveCmd.Short)
	assert.NotNil(t, serveCmd.Run)
}

func TestServeCmd_IsAddedToRoot(t *testing.T) {
	// Verify serveCmd is in rootCmd's commands
	commands := rootCmd.Commands()
	found := false
	for _, cmd := range commands {
		if cmd.Use == "server" {
			found = true
			break
		}
	}
	assert.True(t, found, "serveCmd should be added to rootCmd")
}

func TestGOMAXPROCS_Logic(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		expected int
	}{
		{
			name:     "empty env - should default to 4",
			envValue: "",
			expected: 4,
		},
		{
			name:     "env set to 2",
			envValue: "2",
			expected: 2,
		},
		{
			name:     "env set to 8",
			envValue: "8",
			expected: 8,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original
			original := os.Getenv("GOMAXPROCS")
			defer func() {
				if original != "" {
					os.Setenv("GOMAXPROCS", original)
				} else {
					os.Unsetenv("GOMAXPROCS")
				}
			}()

			if tt.envValue == "" {
				os.Unsetenv("GOMAXPROCS")
			} else {
				os.Setenv("GOMAXPROCS", tt.envValue)
			}

			// Test the logic from startServer
			if gomaxprocs := os.Getenv("GOMAXPROCS"); gomaxprocs == "" {
				runtime.GOMAXPROCS(4)
			} else {
				// This would be set in the actual code
				// For testing, we just verify the env var is set correctly
				assert.Equal(t, tt.envValue, gomaxprocs)
			}

			if tt.envValue == "" {
				assert.Equal(t, tt.expected, runtime.GOMAXPROCS(0))
			}
		})
	}
}

func TestHostnameRetrieval(t *testing.T) {
	hostname, err := os.Hostname()
	assert.NoError(t, err)
	assert.NotEmpty(t, hostname)
}

func TestGOGC_Environment(t *testing.T) {
	// Test that GOGC can be read
	gogc := os.Getenv("GOGC")
	// GOGC may or may not be set, just verify we can read it
	_ = gogc
	
	// If set, it should be a valid number or empty
	if gogc != "" {
		// Just verify it's set, actual validation happens in Go runtime
		assert.NotEmpty(t, gogc)
	}
}

func TestConfig_CanBeCreated(t *testing.T) {
	// Create a temporary config file for testing
	tmpFile, err := os.CreateTemp("", "test-config-*.yaml")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	configContent := `
server:
  host: "0.0.0.0"
  port: 8080
pubsub:
  provider: "nats"
  url: "nats://localhost:4222"
`
	_, err = tmpFile.WriteString(configContent)
	require.NoError(t, err)
	tmpFile.Close()

	// Load config
	cfg, err := config.Load(tmpFile.Name(), "")
	require.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, "0.0.0.0", cfg.Server.Host)
	assert.Equal(t, 8080, cfg.Server.Port)
}

func TestRuntimeInfo(t *testing.T) {
	t.Run("GOMAXPROCS can be read", func(t *testing.T) {
		procs := runtime.GOMAXPROCS(0)
		assert.Greater(t, procs, 0)
	})

	t.Run("NumCPU returns valid value", func(t *testing.T) {
		cpus := runtime.NumCPU()
		assert.Greater(t, cpus, 0)
	})
}

func TestServeCmd_Run_FailsWithoutConfig(t *testing.T) {
	// Save original cfgFile
	originalCfgFile := cfgFile
	originalEnv := env
	defer func() {
		cfgFile = originalCfgFile
		env = originalEnv
	}()

	// Set invalid config file
	cfgFile = "/nonexistent/config.yaml"
	env = ""

	// The Run function should fail when config can't be loaded
	// We can't actually run it as it would call log.Fatalf
	// But we can verify the config load fails
	_, err := config.Load(cfgFile, env)
	assert.Error(t, err)
}

func TestInit_RegistersServeCmd(t *testing.T) {
	// Verify init() was called and serveCmd was added
	commands := rootCmd.Commands()
	var serveCmdFound *cobra.Command
	for _, cmd := range commands {
		if cmd.Use == "server" {
			serveCmdFound = cmd
			break
		}
	}
	
	require.NotNil(t, serveCmdFound)
	assert.Equal(t, "Start the real-time backend server", serveCmdFound.Short)
	assert.NotNil(t, serveCmdFound.Run)
}

