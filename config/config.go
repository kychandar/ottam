package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	Server struct {
		Host string
		Port int
	}
	PubSub struct {
		Provider string
		URL      string
	}
	// Metrics struct {
	// 	Enabled bool
	// 	Port    int
	// }
}

func Load(cfgFile, env string) (*Config, error) {
	v := viper.New()

	// Default values
	v.SetDefault("server.host", "0.0.0.0")
	v.SetDefault("server.port", 8080)

	// If config file passed via CLI flag
	if cfgFile != "" {
		v.SetConfigFile(cfgFile)
	} else {
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath(".")
		v.AddConfigPath("./config")
	}

	// Read main config
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("error reading config: %w", err)
	}

	// Merge environment-specific config (config.prod.yaml, etc.)
	if env != "" {
		v.SetConfigName(fmt.Sprintf("config.%s", env))
		_ = v.MergeInConfig() // optional, ignore error if not found
	}

	// Environment overrides
	v.SetEnvPrefix("MYAPP")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("error unmarshalling config: %w", err)
	}

	return &cfg, nil
}
