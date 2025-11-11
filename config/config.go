package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

type TLSConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	CertFile string `mapstructure:"cert_file"`
	KeyFile  string `mapstructure:"key_file"`
	CAFile   string `mapstructure:"ca_file"`
}

type Config struct {
	Server struct {
		Host            string    `mapstructure:"host"`
		Port            int       `mapstructure:"port"`
		TLS             TLSConfig `mapstructure:"tls"`
		ShutdownTimeout int       `mapstructure:"shutdown_timeout"` // seconds
	} `mapstructure:"server"`
	PubSub struct {
		Provider string    `mapstructure:"provider"`
		URL      string    `mapstructure:"url"`
		TLS      TLSConfig `mapstructure:"tls"`
	} `mapstructure:"pubsub"`
	RedisProtoCache struct {
		Addr []string `mapstructure:"addr"`
	} `mapstructure:"redisProtoCache"`
	Health struct {
		Enabled       bool   `mapstructure:"enabled"`
		Port          int    `mapstructure:"port"`
		ReadinessPath string `mapstructure:"readiness_path"`
		LivenessPath  string `mapstructure:"liveness_path"`
	} `mapstructure:"health"`
}

func Load(cfgFile, env string) (*Config, error) {
	v := viper.New()

	// Default values
	v.SetDefault("server.host", "0.0.0.0")
	v.SetDefault("server.port", 8080)
	v.SetDefault("server.shutdown_timeout", 30)
	v.SetDefault("server.tls.enabled", false)
	v.SetDefault("pubsub.tls.enabled", false)
	v.SetDefault("health.enabled", true)
	v.SetDefault("health.port", 8081)
	v.SetDefault("health.readiness_path", "/health/ready")
	v.SetDefault("health.liveness_path", "/health/live")

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
