package database

import (
	"fmt"
	"strings"

	"channel-adapter-gateway/internal/config"
	"channel-adapter-gateway/internal/model"
	"channel-adapter-gateway/internal/service"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func Open(cfg config.DatabaseConfig) (*gorm.DB, error) {
	switch strings.ToLower(cfg.Driver) {
	case "postgres", "postgresql", "pg":
		return gorm.Open(postgres.Open(cfg.DSN), &gorm.Config{})
	default:
		return nil, fmt.Errorf("unsupported database driver: %s", cfg.Driver)
	}
}

func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&model.User{},
		&model.Provider{},
		&model.MappingRule{},
		&model.RequestLog{},
	)
}

func Seed(db *gorm.DB, cfg *config.Config) error {
	if err := seedAdmin(db, cfg.Admin); err != nil {
		return err
	}
	return nil
}

func seedAdmin(db *gorm.DB, cfg config.AdminConfig) error {
	var count int64
	if err := db.Model(&model.User{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	hash, err := service.HashPassword(cfg.Password)
	if err != nil {
		return err
	}
	return db.Create(&model.User{
		Username:     cfg.Username,
		PasswordHash: hash,
		Role:         "admin",
		Enabled:      true,
	}).Error
}
