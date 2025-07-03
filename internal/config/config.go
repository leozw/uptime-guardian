package config

import (
	"os"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Server    ServerConfig
	Database  DatabaseConfig
	Keycloak  KeycloakConfig
	Mimir     MimirConfig
	Scheduler SchedulerConfig
	Regions   map[string]RegionConfig
}

type ServerConfig struct {
	Port string
	Mode string
}

type DatabaseConfig struct {
	URL            string
	MaxConnections int
	MaxIdleConns   int
}

type KeycloakConfig struct {
	URL          string
	Realm        string
	ClientID     string
	ClientSecret string
}

type MimirConfig struct {
	URL           string
	TenantHeader  string
	BatchSize     int
	FlushInterval time.Duration
	AuthToken     string
}

type SchedulerConfig struct {
	WorkerCount  int
	CheckTimeout time.Duration
	MaxRetries   int
}

type RegionConfig struct {
	Name     string
	Location string
	Provider string
}

func Load() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./config")
	viper.SetEnvPrefix("UPTIME")
	viper.AutomaticEnv()

	// Set defaults
	viper.SetDefault("server.port", "8080")
	viper.SetDefault("server.mode", "debug")
	viper.SetDefault("database.maxconnections", 25)
	viper.SetDefault("database.maxidleconns", 5)
	viper.SetDefault("mimir.tenantheader", "X-Scope-OrgID")
	viper.SetDefault("mimir.batchsize", 1000)
	viper.SetDefault("mimir.flushinterval", "10s")
	viper.SetDefault("scheduler.workercount", 10)
	viper.SetDefault("scheduler.checktimeout", "30s")
	viper.SetDefault("scheduler.maxretries", 3)

	var cfg Config
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, err
		}
	}

	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	// Override with environment variables
	if url := os.Getenv("DATABASE_URL"); url != "" {
		cfg.Database.URL = url
	}
	if url := os.Getenv("KEYCLOAK_URL"); url != "" {
		cfg.Keycloak.URL = url
	}
	if url := os.Getenv("MIMIR_URL"); url != "" {
		cfg.Mimir.URL = url
	}
	if token := os.Getenv("MIMIR_AUTH_TOKEN"); token != "" {
		cfg.Mimir.AuthToken = token
	}

	// Default regions if not configured
	if len(cfg.Regions) == 0 {
		cfg.Regions = map[string]RegionConfig{
			"us-east":  {Name: "US East", Location: "Virginia", Provider: "aws"},
			"eu-west":  {Name: "EU West", Location: "Ireland", Provider: "aws"},
			"asia-pac": {Name: "Asia Pacific", Location: "Singapore", Provider: "aws"},
		}
	}

	return &cfg, nil
}
