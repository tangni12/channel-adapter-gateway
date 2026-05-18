package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	Admin    AdminConfig    `yaml:"admin"`
	Logging  LoggingConfig  `yaml:"logging"`
}

type ServerConfig struct {
	Addr                   string `yaml:"addr"`
	JWTSecret              string `yaml:"jwt_secret"`
	UpstreamTimeoutSeconds int    `yaml:"upstream_timeout_seconds"`
}

type DatabaseConfig struct {
	Driver                 string `yaml:"driver"`
	DSN                    string `yaml:"dsn"`
	AutoMigrate            bool   `yaml:"auto_migrate"`
	MaxOpenConns           int    `yaml:"max_open_conns"`
	MaxIdleConns           int    `yaml:"max_idle_conns"`
	ConnMaxLifetimeSeconds int    `yaml:"conn_max_lifetime_seconds"`
}

type AdminConfig struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

type LoggingConfig struct {
	AsyncRequestLog       *bool `yaml:"async_request_log"`
	QueueSize             int   `yaml:"queue_size"`
	WorkerCount           int   `yaml:"worker_count"`
	BatchSize             int   `yaml:"batch_size"`
	FlushIntervalMs       int   `yaml:"flush_interval_ms"`
	MaxRetries            int   `yaml:"max_retries"`
	RetryIntervalMs       int   `yaml:"retry_interval_ms"`
	EnqueueTimeoutMs      int   `yaml:"enqueue_timeout_ms"`
	MaxPayloadBytes       int   `yaml:"max_payload_bytes"`
	LogDroppedWhenFull    bool  `yaml:"log_dropped_when_full"`
	SyncOnShutdownSeconds int   `yaml:"sync_on_shutdown_seconds"`
}

func (c LoggingConfig) IsAsyncRequestLog() bool {
	return c.AsyncRequestLog == nil || *c.AsyncRequestLog
}

func Load(path string) (*Config, error) {
	if path == "" {
		path = os.Getenv("GATEWAY_CONFIG")
	}
	if path == "" {
		path = filepath.Join("configs", "config.yaml")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse yaml config: %w", err)
	}
	applyDefaults(&cfg)
	return &cfg, nil
}

func applyDefaults(cfg *Config) {
	if cfg.Server.Addr == "" {
		cfg.Server.Addr = ":8088"
	}
	if cfg.Server.JWTSecret == "" {
		cfg.Server.JWTSecret = "please-change-this-secret"
	}
	if cfg.Server.UpstreamTimeoutSeconds <= 0 {
		cfg.Server.UpstreamTimeoutSeconds = 180
	}
	if cfg.Database.Driver == "" {
		cfg.Database.Driver = "postgres"
	}
	if cfg.Database.MaxOpenConns <= 0 {
		cfg.Database.MaxOpenConns = 30
	}
	if cfg.Database.MaxIdleConns <= 0 {
		cfg.Database.MaxIdleConns = 10
	}
	if cfg.Database.ConnMaxLifetimeSeconds <= 0 {
		cfg.Database.ConnMaxLifetimeSeconds = 1800
	}
	if cfg.Admin.Username == "" {
		cfg.Admin.Username = "admin"
	}
	if cfg.Admin.Password == "" {
		cfg.Admin.Password = "admin123456"
	}
	if cfg.Logging.QueueSize <= 0 {
		cfg.Logging.QueueSize = 2000
	}
	if cfg.Logging.WorkerCount <= 0 {
		cfg.Logging.WorkerCount = 2
	}
	if cfg.Logging.BatchSize <= 0 {
		cfg.Logging.BatchSize = 50
	}
	if cfg.Logging.FlushIntervalMs <= 0 {
		cfg.Logging.FlushIntervalMs = 1000
	}
	if cfg.Logging.MaxRetries <= 0 {
		cfg.Logging.MaxRetries = 3
	}
	if cfg.Logging.RetryIntervalMs <= 0 {
		cfg.Logging.RetryIntervalMs = 300
	}
	if cfg.Logging.EnqueueTimeoutMs <= 0 {
		cfg.Logging.EnqueueTimeoutMs = 10
	}
	if cfg.Logging.MaxPayloadBytes <= 0 {
		cfg.Logging.MaxPayloadBytes = 10 * 1024 * 1024
	}
	if cfg.Logging.SyncOnShutdownSeconds <= 0 {
		cfg.Logging.SyncOnShutdownSeconds = 5
	}
}
