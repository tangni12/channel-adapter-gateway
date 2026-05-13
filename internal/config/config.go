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
}

type ServerConfig struct {
	Addr                   string `yaml:"addr"`
	JWTSecret              string `yaml:"jwt_secret"`
	UpstreamTimeoutSeconds int    `yaml:"upstream_timeout_seconds"`
}

type DatabaseConfig struct {
	Driver      string `yaml:"driver"`
	DSN         string `yaml:"dsn"`
	AutoMigrate bool   `yaml:"auto_migrate"`
}

type AdminConfig struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
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
	if cfg.Admin.Username == "" {
		cfg.Admin.Username = "admin"
	}
	if cfg.Admin.Password == "" {
		cfg.Admin.Password = "admin123456"
	}
}
